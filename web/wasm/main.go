//go:build js && wasm

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"syscall/js"

	"github.com/asciimoth/gonnect-netstack/vtun"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect/tun"
)

type app struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	conn   *transport.Conn
	vtun   *vtun.VTun
	p2p    *tun.Point2Point
	client *http.Client
}

func log(messages ...any) {
	messages = append([]any{"[WASM]:"}, messages...)
	fmt.Println(messages...)
}

func logf(format string, messages ...any) {
	fmt.Printf("[WASM]: "+format, messages...)
}

func main() {
	instance := &app{}
	log("initializing wasm bindings")
	js.Global().Set("vpnDemoConnect", promiseFunc(instance.connect))
	js.Global().Set("vpnDemoRequest", promiseFunc(instance.request))
	js.Global().Set("vpnDemoDisconnect", promiseFunc(instance.disconnectJS))

	log("wasm bindings ready")
	select {}
}

func promiseFunc(fn func(args []js.Value) (any, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		handler := js.FuncOf(func(this js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			reject := promiseArgs[1]

			go func() {
				result, err := fn(args)
				if err != nil {
					reject.Invoke(err.Error())
					return
				}
				resolve.Invoke(result)
			}()

			return nil
		})

		promise := js.Global().Get("Promise").New(handler)
		handler.Release()
		return promise
	})
}

func (a *app) connect(args []js.Value) (any, error) {
	if len(args) < 2 {
		logf("connect failed: expected 2 args, got %d\n", len(args))
		return nil, fmt.Errorf("expected ws url and tun addr")
	}

	wsURL := strings.TrimSpace(args[0].String())
	tunAddr := strings.TrimSpace(args[1].String())
	logf("connect requested: ws_url=%q tun_addr=%q\n", wsURL, tunAddr)
	if wsURL == "" {
		log("connect failed: websocket url is required")
		return nil, fmt.Errorf("websocket url is required")
	}

	addr := netip.MustParseAddr("10.200.1.5")
	if tunAddr != "" {
		parsed, err := netip.ParseAddr(tunAddr)
		if err != nil {
			logf("connect failed: invalid tun addr %q: %v\n", tunAddr, err)
			return nil, fmt.Errorf("invalid tun addr: %w", err)
		}
		addr = parsed
	}

	log("disconnecting previous session before reconnect")
	a.disconnect()

	ctx, cancel := context.WithCancel(context.Background())

	conn, err := transport.Dial(ctx, wsURL, nil)
	if err != nil {
		cancel()
		logf("connect failed: dial %q: %v\n", wsURL, err)
		return nil, err
	}

	vt, err := (&vtun.Opts{
		LocalAddrs: []netip.Addr{addr},
		Name:       "web-vtun",
	}).Build()
	if err != nil {
		cancel()
		_ = conn.Close()
		logf("connect failed: build vtun for %s: %v\n", addr, err)
		return nil, err
	}

	p2p := tun.NewP2P(nil)
	p2p.SetA(conn)
	p2p.SetB(vt)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: vt.Dial,
		},
	}

	a.mu.Lock()
	a.cancel = cancel
	a.conn = conn
	a.vtun = vt
	a.p2p = p2p
	a.client = client
	a.mu.Unlock()

	logf("connect succeeded: ws_url=%q vtun_addr=%s\n", wsURL, addr)
	return fmt.Sprintf("Connected to %s with VTun address %s", wsURL, addr), nil
}

func (a *app) request(args []js.Value) (any, error) {
	if len(args) < 4 {
		logf("request failed: expected 4 args, got %d\n", len(args))
		return nil, fmt.Errorf("expected method, url, headers and body")
	}

	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		log("request failed: vpn is not connected")
		return nil, fmt.Errorf("vpn is not connected")
	}

	method := strings.TrimSpace(args[0].String())
	targetURL := strings.TrimSpace(args[1].String())
	headersText := args[2].String()
	bodyText := args[3].String()

	if method == "" {
		method = http.MethodGet
	}
	if targetURL == "" {
		log("request failed: target url is required")
		return nil, fmt.Errorf("target url is required")
	}
	logf("request started: method=%s url=%q\n", method, targetURL)

	req, err := http.NewRequestWithContext(
		context.Background(),
		method,
		targetURL,
		strings.NewReader(bodyText),
	)
	if err != nil {
		logf("request failed: build request: %v\n", err)
		return nil, err
	}

	for _, line := range strings.Split(headersText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			logf("request failed: invalid header line %q\n", line)
			return nil, fmt.Errorf("invalid header line %q", line)
		}

		req.Header.Add(strings.TrimSpace(key), strings.TrimSpace(value))
	}

	resp, err := client.Do(req)
	if err != nil {
		logf("request failed: do request %q: %v\n", targetURL, err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logf("request failed: read response body: %v\n", err)
		return nil, err
	}

	var result bytes.Buffer
	fmt.Fprintf(&result, "%s\n", resp.Status)
	resp.Header.Write(&result)
	result.WriteString("\n")
	result.Write(body)

	logf("request finished: method=%s url=%q status=%s bytes=%d\n", method, targetURL, resp.Status, len(body))
	return result.String(), nil
}

func (a *app) disconnectJS(args []js.Value) (any, error) {
	log("disconnect requested from javascript")
	a.disconnect()
	return nil, nil
}

func (a *app) disconnect() {
	log("disconnect started")
	a.mu.Lock()
	cancel := a.cancel
	conn := a.conn
	vt := a.vtun
	p2p := a.p2p
	a.cancel = nil
	a.conn = nil
	a.vtun = nil
	a.p2p = nil
	a.client = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if p2p != nil {
		p2p.Stop()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if vt != nil {
		_ = vt.Close()
	}
	log("disconnect finished")
}
