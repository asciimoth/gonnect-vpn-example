package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/device"
	"github.com/asciimoth/gonnect-vpn-example/helpers"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/runner"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Gonnect VPN Example"),
			app.Size(unit.Dp(1180), unit.Dp(860)),
		)
		ui := newGUI(w)
		if err := ui.run(); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

type gui struct {
	window *app.Window
	ops    op.Ops
	theme  *material.Theme

	rootCtx    context.Context
	rootCancel context.CancelFunc

	updates    chan uiUpdate
	logger     logger.Logger
	logs       *logBuffer
	lastEvent  string
	stdoutLogs *log.Logger

	session  *runner.Session
	starting bool
	stopping bool
	status   string

	mode    widget.Enum
	tunType widget.Enum

	startBtn widget.Clickable
	stopBtn  widget.Clickable

	logView     widget.Editor
	controlList widget.List

	connect      widget.Editor
	serve        widget.Editor
	tunName      widget.Editor
	tunAddr      widget.Editor
	tunSubnet    widget.Editor
	tunHTTPAddr  widget.Editor
	tunSocksAddr widget.Editor
}

type uiUpdate struct {
	err     error
	session *runner.Session
	stopped bool
	status  string
}

func newGUI(window *app.Window) *gui {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	logs := newLogBuffer(600, window.Invalidate)
	stdoutLogs := log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds)
	ui := &gui{
		window:     window,
		theme:      material.NewTheme(),
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
		updates:    make(chan uiUpdate, 32),
		logs:       logs,
		stdoutLogs: stdoutLogs,
		status:     "idle",
		lastEvent:  "ready",
	}
	ui.logger = &teeLogger{
		loggers: []logger.Logger{
			stdoutLogs,
			&screenLogger{buffer: logs},
		},
	}

	ui.mode.Value = "server"
	ui.tunType.Value = "vtun+http"

	ui.controlList.Axis = layout.Vertical
	ui.logView.ReadOnly = true
	ui.logView.SingleLine = false

	ui.connect.SingleLine = true
	ui.connect.SetText("ws://127.0.0.1:8080/ws-vpn")
	ui.serve.SingleLine = true
	ui.serve.SetText("127.0.0.1:8080")
	ui.tunName.SingleLine = true
	ui.tunAddr.SingleLine = true
	ui.tunSubnet.SingleLine = true
	ui.tunHTTPAddr.SingleLine = true
	ui.tunSocksAddr.SingleLine = true
	ui.tunSocksAddr.SetText("127.0.0.1:1080")
	ui.logs.add("INFO", "gui ready")

	return ui
}

func (u *gui) run() error {
	defer func() {
		u.rootCancel()
		if u.session != nil {
			u.session.Stop()
			u.session.Wait()
		}
	}()

	for {
		switch e := u.window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&u.ops, e)
			u.handleActions(gtx)
			u.applyUpdates()
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *gui) handleActions(gtx layout.Context) {
	for u.startBtn.Clicked(gtx) {
		u.startSession()
	}
	for u.stopBtn.Clicked(gtx) {
		u.stopSession()
	}
}

func (u *gui) startSession() {
	if u.starting || u.session != nil {
		return
	}

	conf := u.currentConfig()
	u.starting = true
	u.stopping = false
	u.status = "starting"
	u.lastEvent = "starting vpn session"
	u.logs.add("INFO", "starting vpn session")

	go func() {
		session, err := runner.Start(u.rootCtx, conf, u.logger)
		if err != nil {
			u.enqueue(uiUpdate{
				err:    err,
				status: "failed",
			})
			return
		}

		u.enqueue(uiUpdate{
			session: session,
			status:  "running",
		})

		go func() {
			session.Wait()
			u.enqueue(uiUpdate{
				stopped: true,
				status:  "stopped",
			})
		}()
	}()
}

func (u *gui) stopSession() {
	if u.session == nil || u.stopping {
		return
	}
	u.stopping = true
	u.starting = false
	u.status = "stopping"
	u.lastEvent = "stopping vpn session"
	u.logs.add("INFO", "stopping vpn session")
	u.session.Stop()
}

func (u *gui) enqueue(update uiUpdate) {
	select {
	case u.updates <- update:
	default:
		u.logs.add("WARN", "ui update dropped because the queue is full")
	}
	u.window.Invalidate()
}

func (u *gui) applyUpdates() {
	for {
		select {
		case update := <-u.updates:
			if update.err != nil {
				u.logs.add("ERROR", update.err.Error())
				u.session = nil
				u.starting = false
				u.stopping = false
				u.status = update.status
				u.lastEvent = update.err.Error()
				continue
			}
			if update.session != nil {
				u.session = update.session
				u.starting = false
				u.stopping = false
				u.status = update.status
				u.lastEvent = "vpn session is running"
				u.logs.add("INFO", "vpn session is running")
				continue
			}
			if update.stopped {
				u.session = nil
				u.starting = false
				u.stopping = false
				u.status = update.status
				u.lastEvent = "vpn session stopped"
				u.logs.add("INFO", "vpn session stopped")
			}
		default:
			return
		}
	}
}

func (u *gui) currentConfig() *cfg.Cfg {
	conf := &cfg.Cfg{
		TunType:      strings.TrimSpace(u.tunType.Value),
		TunName:      strings.TrimSpace(u.tunName.Text()),
		TunAddr:      strings.TrimSpace(u.tunAddr.Text()),
		TunSubnet:    strings.TrimSpace(u.tunSubnet.Text()),
		TunHttpAddr:  strings.TrimSpace(u.tunHTTPAddr.Text()),
		TunSocksAddr: strings.TrimSpace(u.tunSocksAddr.Text()),
	}

	if u.mode.Value == "client" {
		conf.Connect = strings.TrimSpace(u.connect.Text())
	} else {
		conf.Serve = strings.TrimSpace(u.serve.Text())
	}

	return conf
}

func (u *gui) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, color.NRGBA{R: 243, G: 245, B: 247, A: 255})

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(
			gtx,
			layout.Rigid(u.layoutHeader),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(0.56, u.layoutLogsPanel),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(0.44, u.layoutControlsPanel),
		)
	})
}

func (u *gui) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.H4(u.theme, "Gonnect VPN Example")
			label.Color = color.NRGBA{R: 25, G: 31, B: 39, A: 255}
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(u.theme, fmt.Sprintf(
				"Status: %s | Mode: %s | TUN: %s",
				u.status,
				u.mode.Value,
				u.tunType.Value,
			))
			label.Color = color.NRGBA{R: 70, G: 81, B: 94, A: 255}
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutStatusBanner),
	)
}

func (u *gui) layoutStatusBanner(gtx layout.Context) layout.Dimensions {
	bg := color.NRGBA{R: 224, G: 231, B: 238, A: 255}
	fg := color.NRGBA{R: 34, G: 41, B: 48, A: 255}

	switch u.status {
	case "running":
		bg = color.NRGBA{R: 220, G: 239, B: 226, A: 255}
		fg = color.NRGBA{R: 38, G: 92, B: 53, A: 255}
	case "starting":
		bg = color.NRGBA{R: 227, G: 237, B: 249, A: 255}
		fg = color.NRGBA{R: 34, G: 76, B: 129, A: 255}
	case "stopping":
		bg = color.NRGBA{R: 244, G: 233, B: 220, A: 255}
		fg = color.NRGBA{R: 138, G: 85, B: 24, A: 255}
	case "failed":
		bg = color.NRGBA{R: 250, G: 225, B: 225, A: 255}
		fg = color.NRGBA{R: 149, G: 38, B: 38, A: 255}
	}

	return widget.Border{
		Color:        color.NRGBA{R: 204, G: 212, B: 220, A: 255},
		CornerRadius: unit.Dp(10),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(unit.Dp(10))).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, bg)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(u.theme, u.lastEvent)
					label.Color = fg
					return label.Layout(gtx)
				})
			},
		)
	})
}

func (u *gui) layoutLogsPanel(gtx layout.Context) layout.Dimensions {
	lines := u.logs.snapshot()
	logText := "No logs yet."
	if len(lines) > 0 {
		logText = strings.Join(lines, "\n")
	}
	if u.logView.Text() != logText {
		u.logView.SetText(logText)
		u.logView.SetCaret(u.logView.Len(), u.logView.Len())
	}

	return u.layoutPanel(gtx, "Logs", func(gtx layout.Context) layout.Dimensions {
		border := widget.Border{
			Color:        color.NRGBA{R: 225, G: 229, B: 234, A: 255},
			CornerRadius: unit.Dp(8),
			Width:        unit.Dp(1),
		}
		return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			minHeight := gtx.Dp(unit.Dp(180))
			if gtx.Constraints.Min.Y < minHeight {
				gtx.Constraints.Min.Y = minHeight
			}
			return layout.Background{}.Layout(
				gtx,
				func(gtx layout.Context) layout.Dimensions {
					defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(unit.Dp(8))).Push(gtx.Ops).Pop()
					paint.Fill(gtx.Ops, color.NRGBA{R: 244, G: 247, B: 249, A: 255})
					return layout.Dimensions{Size: gtx.Constraints.Min}
				},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(u.theme, &u.logView, "")
						editor.Color = color.NRGBA{R: 28, G: 33, B: 42, A: 255}
						return editor.Layout(gtx)
					})
				},
			)
		})
	})
}

func (u *gui) layoutControlsPanel(gtx layout.Context) layout.Dimensions {
	return u.layoutPanel(gtx, "Controls", func(gtx layout.Context) layout.Dimensions {
		return material.List(u.theme, &u.controlList).Layout(gtx, 10, func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				switch index {
				case 0:
					return u.layoutModeRow(gtx)
				case 1:
					return u.layoutTunTypeRow(gtx)
				case 2:
					return u.layoutField(gtx, "Server address", &u.serve, "127.0.0.1:8080")
				case 3:
					return u.layoutField(gtx, "Connect URL", &u.connect, "ws://127.0.0.1:8080/ws-vpn")
				case 4:
					return u.layoutField(gtx, "TUN name", &u.tunName, "tun0 or custom userspace name")
				case 5:
					return u.layoutField(gtx, "TUN address", &u.tunAddr, "10.200.1.3 or 10.200.1.2/24")
				case 6:
					return u.layoutField(gtx, "TUN subnet", &u.tunSubnet, "10.200.1.0/24")
				case 7:
					return u.layoutField(gtx, "vtun+http bind", &u.tunHTTPAddr, "10.200.1.3:80")
				case 8:
					return u.layoutField(gtx, "vtun+socks bind", &u.tunSocksAddr, "127.0.0.1:1080")
				default:
					return u.layoutActions(gtx)
				}
			})
		})
	})
}

func (u *gui) layoutModeRow(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(u.theme, "Mode")
			label.Color = color.NRGBA{R: 48, G: 57, B: 66, A: 255}
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(u.theme, &u.mode, "server", "Server").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(18)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(u.theme, &u.mode, "client", "Client").Layout(gtx)
				}),
			)
		}),
	)
}

func (u *gui) layoutTunTypeRow(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(u.theme, "TUN backend")
			label.Color = color.NRGBA{R: 48, G: 57, B: 66, A: 255}
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(u.theme, &u.tunType, "native", "native").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(u.theme, &u.tunType, "vtun+http", "vtun+http").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(u.theme, &u.tunType, "vtun+socks", "vtun+socks").Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.tunType.Value == "native" && !helpers.IsAdmin() {
				label := material.Caption(u.theme, "native requires admin privileges on this machine")
				label.Color = color.NRGBA{R: 176, G: 70, B: 28, A: 255}
				return label.Layout(gtx)
			}
			if !device.IsPrivileged(u.tunType.Value) {
				label := material.Caption(u.theme, "userspace vtun backends do not require admin privileges")
				label.Color = color.NRGBA{R: 78, G: 114, B: 58, A: 255}
				return label.Layout(gtx)
			}
			return layout.Dimensions{}
		}),
	)
}

func (u *gui) layoutField(
	gtx layout.Context,
	labelText string,
	editor *widget.Editor,
	hint string,
) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(u.theme, labelText)
			label.Color = color.NRGBA{R: 48, G: 57, B: 66, A: 255}
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			border := widget.Border{
				Color:        color.NRGBA{R: 202, G: 209, B: 216, A: 255},
				CornerRadius: unit.Dp(8),
				Width:        unit.Dp(1),
			}
			return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Editor(u.theme, editor, hint).Layout)
			})
		}),
	)
}

func (u *gui) layoutActions(gtx layout.Context) layout.Dimensions {
	startLabel := "Start"
	if u.starting {
		startLabel = "Starting..."
	} else if u.session != nil {
		startLabel = "Running"
	}

	stopLabel := "Stop"
	if u.stopping {
		stopLabel = "Stopping..."
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(u.theme, &u.startBtn, startLabel)
					if u.starting || u.session != nil {
						button.Background = color.NRGBA{R: 150, G: 181, B: 206, A: 255}
					}
					return button.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(u.theme, &u.stopBtn, stopLabel)
					button.Background = color.NRGBA{R: 96, G: 107, B: 120, A: 255}
					if u.session == nil {
						button.Background = color.NRGBA{R: 153, G: 160, B: 168, A: 255}
					}
					return button.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := "Choose server or client mode, select a TUN backend, then start the session."
			if u.mode.Value == "client" {
				text = "Client mode uses the Connect URL. Server address is ignored until you switch back."
			}
			if u.mode.Value == "server" {
				text = "Server mode listens on the server address and serves /ws-vpn for VPN peers."
			}
			label := material.Caption(u.theme, text)
			label.Color = color.NRGBA{R: 88, G: 98, B: 110, A: 255}
			return label.Layout(gtx)
		}),
	)
}

func (u *gui) layoutPanel(
	gtx layout.Context,
	title string,
	content layout.Widget,
) layout.Dimensions {
	border := widget.Border{
		Color:        color.NRGBA{R: 214, G: 219, B: 224, A: 255},
		CornerRadius: unit.Dp(12),
		Width:        unit.Dp(1),
	}

	return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(unit.Dp(12))).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 252, A: 255})
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(
						gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.H6(u.theme, title)
							label.Color = color.NRGBA{R: 34, G: 41, B: 48, A: 255}
							return label.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Flexed(1, content),
					)
				})
			},
		)
	})
}

type logBuffer struct {
	mu       sync.RWMutex
	lines    []string
	limit    int
	notifyUI func()
}

func newLogBuffer(limit int, notifyUI func()) *logBuffer {
	return &logBuffer{
		limit:    limit,
		notifyUI: notifyUI,
	}
}

func (b *logBuffer) add(level string, message string) {
	line := fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05.000"), level, message)

	b.mu.Lock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.limit {
		b.lines = append([]string(nil), b.lines[len(b.lines)-b.limit:]...)
	}
	b.mu.Unlock()

	if b.notifyUI != nil {
		b.notifyUI()
	}
}

func (b *logBuffer) snapshot() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

type screenLogger struct {
	buffer *logBuffer
}

var _ logger.Logger = (*screenLogger)(nil)

type teeLogger struct {
	loggers []logger.Logger
}

var _ logger.Logger = (*teeLogger)(nil)

func (l *screenLogger) Print(v ...any) {
	l.buffer.add("INFO", fmt.Sprint(v...))
}

func (l *screenLogger) Println(v ...any) {
	l.buffer.add("INFO", strings.TrimSpace(fmt.Sprintln(v...)))
}

func (l *screenLogger) Printf(format string, v ...any) {
	l.buffer.add("INFO", fmt.Sprintf(format, v...))
}

func (l *screenLogger) Fatal(v ...any) {
	l.buffer.add("FATAL", fmt.Sprint(v...))
}

func (l *screenLogger) Fatalf(format string, v ...any) {
	l.buffer.add("FATAL", fmt.Sprintf(format, v...))
}

func (l *screenLogger) Fatalln(v ...any) {
	l.buffer.add("FATAL", strings.TrimSpace(fmt.Sprintln(v...)))
}

func (l *screenLogger) Panic(v ...any) {
	l.buffer.add("PANIC", fmt.Sprint(v...))
}

func (l *screenLogger) Panicf(format string, v ...any) {
	l.buffer.add("PANIC", fmt.Sprintf(format, v...))
}

func (l *screenLogger) Panicln(v ...any) {
	l.buffer.add("PANIC", strings.TrimSpace(fmt.Sprintln(v...)))
}

func (l *teeLogger) Print(v ...any) {
	for _, target := range l.loggers {
		target.Print(v...)
	}
}

func (l *teeLogger) Println(v ...any) {
	for _, target := range l.loggers {
		target.Println(v...)
	}
}

func (l *teeLogger) Printf(format string, v ...any) {
	for _, target := range l.loggers {
		target.Printf(format, v...)
	}
}

func (l *teeLogger) Fatal(v ...any) {
	for _, target := range l.loggers {
		target.Fatal(v...)
	}
}

func (l *teeLogger) Fatalf(format string, v ...any) {
	for _, target := range l.loggers {
		target.Fatalf(format, v...)
	}
}

func (l *teeLogger) Fatalln(v ...any) {
	for _, target := range l.loggers {
		target.Fatalln(v...)
	}
}

func (l *teeLogger) Panic(v ...any) {
	for _, target := range l.loggers {
		target.Panic(v...)
	}
}

func (l *teeLogger) Panicf(format string, v ...any) {
	for _, target := range l.loggers {
		target.Panicf(format, v...)
	}
}

func (l *teeLogger) Panicln(v ...any) {
	for _, target := range l.loggers {
		target.Panicln(v...)
	}
}
