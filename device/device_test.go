package device

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/socksgo"
)

func TestVTunSocksReachesPeerHTTPWithDefaultAddresses(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	serverVTun, err := vtunFromCfg(&cfg.Cfg{Serve: "127.0.0.1:9090"})
	if err != nil {
		t.Fatal(err)
	}
	defer serverVTun.Close() //nolint:errcheck

	clientVTun, err := vtunFromCfg(&cfg.Cfg{Connect: "ws://127.0.0.1:9090/ws-vpn"})
	if err != nil {
		t.Fatal(err)
	}
	defer clientVTun.Close() //nolint:errcheck

	serverAddr := serverVTun.LocalAddrs()[0].String()
	clientAddr := clientVTun.LocalAddrs()[0].String()
	if serverAddr != "10.200.1.2" {
		t.Fatalf("unexpected server vtun addr: %s", serverAddr)
	}
	if clientAddr != "10.200.2.1" {
		t.Fatalf("unexpected client vtun addr: %s", clientAddr)
	}

	link := tun.NewP2P(nil)
	defer link.Stop()
	link.SetA(serverVTun)
	link.SetB(clientVTun)

	httpListener, err := clientVTun.ListenTCP(ctx, "tcp4", net.JoinHostPort(clientAddr, "80"))
	if err != nil {
		t.Fatal(err)
	}
	defer httpListener.Close() //nolint:errcheck

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "Hello from userspace TCP!")
		}),
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = httpServer.Serve(httpListener)
	})
	defer func() {
		_ = httpServer.Close()
		wg.Wait()
	}()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxyListener.Close() //nolint:errcheck

	socksServer := &socksgo.Server{
		Dialer:         serverVTun.Dial,
		Listener:       serverVTun.Listen,
		PacketDialer:   serverVTun.PacketDial,
		PacketListener: serverVTun.ListenPacket,
	}
	wg.Go(func() {
		for {
			conn, err := proxyListener.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer conn.Close() //nolint:errcheck
				_ = socksServer.Accept(ctx, conn, false)
			})
		}
	})

	proxyClient, err := socksgo.ClientFromURL("socks5://" + proxyListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reqCancel()

	conn, err := proxyClient.Dial(reqCtx, "tcp4", net.JoinHostPort(clientAddr, "80"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck

	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: "+clientAddr+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp), "Hello from userspace TCP!") {
		t.Fatalf("unexpected proxy response: %s", resp)
	}
}

func TestTunFromCfgVTunSocks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	dev, err := TunFromCfg(
		ctx,
		&cfg.Cfg{
			Serve:        "127.0.0.1:9090",
			TunType:      "vtun+socks",
			TunSocksAddr: "127.0.0.1:0",
		},
		&wg,
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = dev.Close()
		cancel()
		wg.Wait()
	}()
}
