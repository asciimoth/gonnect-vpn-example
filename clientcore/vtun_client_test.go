package clientcore

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asciimoth/gonnect-netstack/vtun"
	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect/tun"
)

func TestParseHeaderLines(t *testing.T) {
	headers, err := parseHeaderLines("Accept: text/plain\nX-Test: one\nX-Test: two\n")
	if err != nil {
		t.Fatalf("parseHeaderLines returned error: %v", err)
	}

	if got := headers.Get("Accept"); got != "text/plain" {
		t.Fatalf("unexpected Accept header: %q", got)
	}

	values := headers.Values("X-Test")
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("unexpected X-Test headers: %#v", values)
	}
}

func TestParseHeaderLinesRejectsInvalidLine(t *testing.T) {
	if _, err := parseHeaderLines("broken-header"); err == nil {
		t.Fatal("expected invalid header error")
	}
}

func TestFormatHTTPResponse(t *testing.T) {
	resp := &http.Response{
		Status: "200 OK",
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
			"X-Test":       []string{"1"},
		},
	}

	got := formatHTTPResponse(resp, []byte("hello"))
	if got == "" {
		t.Fatal("expected formatted response")
	}
	if want := "200 OK\n"; got[:len(want)] != want {
		t.Fatalf("response prefix mismatch: %q", got)
	}
	if want := "hello"; got[len(got)-len(want):] != want {
		t.Fatalf("response body mismatch: %q", got)
	}
}

func TestVTunClientSessionRequestAndPing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	serverVTun, err := (&vtun.Opts{
		LocalAddrs: []netip.Addr{netip.MustParseAddr("10.200.1.3")},
		Name:       "server-vtun",
	}).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer serverVTun.Close() //nolint:errcheck

	<-serverVTun.Events()

	httpListener, err := serverVTun.ListenTCP(ctx, "tcp4", "10.200.1.3:80")
	if err != nil {
		t.Fatal(err)
	}
	defer httpListener.Close() //nolint:errcheck

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "vtun")
			_, _ = io.WriteString(w, "Hello from vtun server!")
		}),
	}

	var wg sync.WaitGroup
	var wsServer *http.Server
	defer func() {
		cancel()
		if wsServer != nil {
			_ = wsServer.Shutdown(context.Background())
		}
		_ = httpServer.Close()
		wg.Wait()
	}()

	wg.Go(func() {
		_ = httpServer.Serve(httpListener)
	})

	wsListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer wsListener.Close() //nolint:errcheck

	wsServer = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/ws-vpn" {
				http.NotFound(w, r)
				return
			}

			conn, err := transport.Accept(ctx, w, r)
			if err != nil {
				t.Errorf("transport accept failed: %v", err)
				return
			}

			link := tun.NewP2P(nil)
			link.SetA(serverVTun)
			link.SetB(conn)

			wg.Go(func() {
				defer link.Stop()
				defer conn.Close() //nolint:errcheck
				<-ctx.Done()
			})
		}),
	}
	wg.Go(func() {
		if err := wsServer.Serve(wsListener); err != nil &&
			err != http.ErrServerClosed &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			t.Errorf("ws server failed: %v", err)
		}
	})

	session, err := NewVTunClientSession(ctx, &cfg.Cfg{
		Connect: "ws://" + wsListener.Addr().String() + "/ws-vpn",
		TunAddr: "10.200.1.5",
		TunName: "android-vtun-test",
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		session.Stop()
		session.Wait()
	}()

	requestCtx, requestCancel := context.WithTimeout(ctx, 5*time.Second)
	defer requestCancel()

	response, err := session.DoRequest(requestCtx, http.MethodGet, "http://10.200.1.3/", "X-Demo: true", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "200 OK") {
		t.Fatalf("unexpected response status: %q", response)
	}
	if !strings.Contains(response, "X-Test: vtun") {
		t.Fatalf("response headers missing X-Test: %q", response)
	}
	if !strings.Contains(response, "Hello from vtun server!") {
		t.Fatalf("response body mismatch: %q", response)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	result, err := session.Ping(pingCtx, "10.200.1.3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "bytes from 10.200.1.3") {
		t.Fatalf("unexpected ping result: %q", result)
	}
}

func TestNewVTunClientSessionRequiresConnectURL(t *testing.T) {
	_, err := NewVTunClientSession(context.Background(), &cfg.Cfg{}, &logger.TestingLogger{T: t})
	if err == nil {
		t.Fatal("expected error for missing connect url")
	}
	if !strings.Contains(err.Error(), "connect url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
