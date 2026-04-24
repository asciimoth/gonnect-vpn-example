package transport

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/asciimoth/gonnect"
	"github.com/asciimoth/gonnect/helpers"
	"github.com/asciimoth/gonnect/tun"
	"github.com/coder/websocket"
)

// Static type assertion
var _ tun.Tun = &Conn{}

func wrapEOF(err error) error {
	if err == nil {
		return nil
	}
	if helpers.ClosedNetworkErrToNil(err) == nil {
		return io.EOF
	}
	return err
}

func Dial(ctx context.Context, url string, dialer gonnect.Dial) (*Conn, error) {
	return DialWithConfig(ctx, url, DialConfig{
		Dialer: dialer,
	})
}

func DialWithConfig(ctx context.Context, url string, cfg DialConfig) (*Conn, error) {
	opts, err := dialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	ws, resp, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	ev := make(chan tun.Event, 42)
	ev <- tun.EventUp

	return &Conn{
		WS:    ws,
		Ctx:   ctx,
		RAddr: url,
		Ev:    ev,
	}, nil
}

func Accept(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
) (*Conn, error) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// This repository is an example app and intentionally allows browser
		// clients served from other origins, such as GitHub Pages.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}

	raddr := "remote client"
	if r != nil {
		if r.RemoteAddr != "" {
			raddr = r.RemoteAddr
		}
	}

	ev := make(chan tun.Event, 42)
	ev <- tun.EventUp

	return &Conn{
		WS:    ws,
		Ctx:   ctx,
		RAddr: raddr,
		Ev:    ev,
	}, nil
}

type Conn struct {
	WS    *websocket.Conn
	Ctx   context.Context // nolint
	RAddr string
	Ev    chan tun.Event
}

func (c *Conn) MWO() int {
	return 0
}

func (c *Conn) MRO() int {
	return 0
}

func (c *Conn) Read(
	bufs [][]byte, sizes []int, offset int,
) (n int, err error) {
	if len(bufs) < 1 {
		return 0, nil
	}
	for {
		typ, msg, err := c.WS.Read(c.Ctx)
		if err != nil {
			return 0, wrapEOF(err)
		}
		if typ != websocket.MessageBinary {
			continue
		}
		sizes[0] = copy(bufs[0][offset:], msg)
		return 1, nil
	}
}

func (c *Conn) Write(bufs [][]byte, offset int) (int, error) {
	for i, b := range bufs {
		err := c.WS.Write(c.Ctx, websocket.MessageBinary, b[offset:])
		if err != nil {
			return i, wrapEOF(err)
		}
	}
	return len(bufs), nil
}

func (c *Conn) File() *os.File {
	return nil
}

func (c *Conn) MTU() (int, error) {
	return 1500, nil
}

func (c *Conn) Name() (string, error) {
	return c.RAddr, nil
}

func (c *Conn) Close() error {
	c.Ev <- tun.EventDown
	return c.WS.Close(websocket.StatusNormalClosure, "")
}

func (c *Conn) BatchSize() int {
	return 1
}

func (c *Conn) Events() <-chan tun.Event {
	return c.Ev
}
