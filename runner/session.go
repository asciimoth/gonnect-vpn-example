package runner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/device"
	"github.com/asciimoth/gonnect-vpn-example/helpers"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect-vpn-example/web"
	"github.com/asciimoth/gonnect/tun"
)

type Session struct {
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
}

func Start(parent context.Context, conf *cfg.Cfg, logger logger.Logger) (*Session, error) {
	if err := cfg.Validate(conf); err != nil {
		return nil, err
	}
	if device.IsPrivileged(conf.TunType) && !helpers.IsAdmin() {
		return nil, fmt.Errorf("%q tun type needs admin privileges", conf.TunType)
	}

	ctx, cancel := context.WithCancel(parent)
	session := &Session{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	dev, err := device.TunFromCfg(ctx, conf, &session.wg, logger)
	if err != nil {
		cancel()
		return nil, err
	}

	session.wg.Go(func() {
		<-ctx.Done()
		_ = dev.Close()
	})

	if conf.Connect != "" {
		err = startClient(ctx, conf, &session.wg, logger, dev)
	} else {
		err = startServer(ctx, conf, &session.wg, logger, dev)
	}
	if err != nil {
		session.Stop()
		_ = dev.Close()
		session.wg.Wait()
		return nil, err
	}

	go func() {
		session.wg.Wait()
		close(session.done)
	}()

	return session, nil
}

func (s *Session) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.cancel()
	})
}

func (s *Session) Done() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.done
}

func (s *Session) Wait() {
	if s == nil {
		return
	}
	<-s.done
}

func startClient(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
	dev tun.Tun,
) error {
	t, err := transport.Dial(ctx, cfg.Connect, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", cfg.Connect, err)
	}
	logger.Printf("connected to %s", cfg.Connect)

	p2p := tun.NewP2P(nil)
	p2p.SetA(t)
	p2p.SetB(dev)

	wg.Go(func() {
		<-ctx.Done()
		p2p.Stop()
		_ = t.Close()
	})

	return nil
}

func startServer(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
	dev tun.Tun,
) error {
	p2p := tun.NewP2P(nil)
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
		p2p.Stop()
		return fmt.Errorf("failed to start listener at %s: %w", cfg.Serve, err)
	}

	server := &http.Server{
		Addr:    cfg.Serve,
		Handler: mux,
	}

	wg.Go(func() {
		<-ctx.Done()
		p2p.Stop()
		_ = server.Close()
	})

	wg.Go(func() {
		logger.Printf("vpn server listening on ws://%s/ws-vpn", cfg.Serve)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Printf("vpn server stopped: %v", err)
		}
	})

	return nil
}
