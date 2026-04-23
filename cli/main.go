package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/device"
	"github.com/asciimoth/gonnect-vpn-example/helpers"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect-vpn-example/web"
	"github.com/asciimoth/gonnect/tun"
)

func main() {
	var logger logger.Logger = log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds)
	cfg := cfg.Load()

	if device.IsPrivileged(cfg.TunType) && !helpers.IsAdmin() {
		logger.Fatalf("%q tun type need admin privileges\n", cfg.TunType)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dev, err := device.TunFromCfg(ctx, cfg, &wg, logger)
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	// TODO: Add support for socks CONNECT/BIND proxies
	if cfg.Connect != "" {
		client(ctx, cfg, &wg, logger, dev)
	} else {
		server(ctx, cfg, &wg, logger, dev)
	}
}

func client(
	ctx context.Context,
	cfg *cfg.Cfg,
	_ *sync.WaitGroup,
	logger logger.Logger,
	dev tun.Tun,
) {
	t, err := transport.Dial(ctx, cfg.Connect, nil)
	if err != nil {
		logger.Printf("failed to connect to %s: %s", cfg.Connect, err)
		return
	}
	logger.Printf("connected to %s", cfg.Connect)

	p2p := tun.NewP2P(nil)
	defer p2p.Stop()
	p2p.SetA(t)
	p2p.SetB(dev)

	<-ctx.Done()
}

func server(
	ctx context.Context,
	cfg *cfg.Cfg,
	_ *sync.WaitGroup,
	logger logger.Logger,
	dev tun.Tun,
) {
	p2p := tun.NewP2P(nil)
	defer p2p.Stop()
	p2p.SetB(dev)

	mux := http.NewServeMux()
	mux.Handle("/", web.Handler())
	mux.HandleFunc("/ws-vpn", func(w http.ResponseWriter, r *http.Request) {
		t, err := transport.Accept(ctx, w, r)
		if err != nil {
			logger.Println("failed to accept transport conn:", err)
			return
		}
		logger.Println("accepted transport conn from:", r.RemoteAddr)
		p2p.SetA(t)
	})

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Serve)
	if err != nil {
		logger.Printf("failed to start listener at %s: %s", cfg.Connect, err)
		return
	}

	server := &http.Server{
		Addr:    cfg.Serve,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	logger.Printf("vpn server listening on ws://%s/ws-vpn", cfg.Serve)
	if err := server.Serve(listener); err != http.ErrServerClosed {
		logger.Printf("vpn server stopped: %v", err)
	}
}
