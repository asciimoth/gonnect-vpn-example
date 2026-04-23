package device

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sync"

	"github.com/asciimoth/gonnect-netstack/vtun"
	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/socksgo"
)

func IsPrivileged(name string) bool {
	switch name {
	case "vtun+http":
		return false
	case "vtun+socks":
		return false
	default:
		return true
	}
}

func wrap(dev tun.Tun, logger logger.Logger) tun.Tun {
	return &tun.CallbackTUN{
		Tun: dev,
		OnRead: func(n int, err error) {
			if err != nil {
				return
			}
			logger.Print("tun <-IP-- transport")
		},
		OnWrite: func(n int, err error) {
			if err != nil {
				return
			}
			logger.Print("tun --IP-> transport")
		},
	}
}

func TunFromCfg(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
) (tun.Tun, error) {
	switch cfg.TunType {
	case "vtun+http":
		dev, err := vtunHttpFromCfg(ctx, cfg, wg, logger)
		if err != nil {
			return nil, err
		}
		return wrap(dev, logger), nil
	case "vtun+socks":
		dev, err := vtunSocksFromCfg(ctx, cfg, wg, logger)
		if err != nil {
			return nil, err
		}
		return wrap(dev, logger), nil
	case "native":
		dev, err := nativeFromCfg(ctx, cfg, wg, logger)
		if err != nil {
			return nil, err
		}
		return wrap(dev, logger), nil
	default:
		return nil, fmt.Errorf("unknown tun type %q", cfg.TunType)
	}
}

func defaultVTunAddr(cfg *cfg.Cfg) netip.Addr {
	// Match the native defaults so each side owns a different subnet endpoint.
	// Using the same address on both peers makes vtun treat the target as local.
	if cfg != nil && cfg.Serve != "" {
		return netip.MustParseAddr("10.200.1.2")
	}
	return netip.MustParseAddr("10.200.2.1")
}

func vtunFromCfg(cfg *cfg.Cfg) (*vtun.VTun, error) {
	laddrs := []netip.Addr{
		defaultVTunAddr(cfg),
	}
	if cfg.TunAddr != "" {
		laddr, err := netip.ParseAddr(cfg.TunAddr)
		if err != nil {
			return nil, err
		}
		laddrs = []netip.Addr{laddr}
	}
	opts := vtun.Opts{
		LocalAddrs: laddrs,
		Name:       cfg.TunName,
	}
	dev, err := opts.Build()
	if err != nil {
		return nil, err
	}
	return dev, nil
}

func vtunSocksFromCfg(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
) (tun.Tun, error) {
	vtun, err := vtunFromCfg(cfg)
	if err != nil {
		return nil, err
	}

	addr := "127.0.0.1:1080"
	if cfg.TunSocksAddr != "" {
		addr = cfg.TunSocksAddr
	}

	server := &socksgo.Server{
		// TODO: Cmd logger
		Dialer:         vtun.Dial,
		Listener:       vtun.Listen,
		PacketDialer:   vtun.PacketDial,
		PacketListener: vtun.ListenPacket,
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		_ = vtun.Close()
		return nil, err
	}
	logger.Printf("local socks over vtun server listening on %s", addr)
	wg.Go(func() {
		<-ctx.Done()
		logger.Println("closing socks over vtun")
		_ = listener.Close()
	})

	wg.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Listener was closed (by context cancellation or error)
				return
			}

			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				logger.Printf("socks connection from %s", conn.RemoteAddr())
				if err := server.Accept(ctx, conn, false); err != nil {
					logger.Printf(
						"socks connection from %s closed with error: %v",
						conn.RemoteAddr(),
						err,
					)
				} else {
					logger.Printf(
						"socks connection from %s closed",
						conn.RemoteAddr(),
					)
				}
			})
		}
	})

	return vtun, nil
}

func vtunHttpFromCfg(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
) (tun.Tun, error) {
	vtun, err := vtunFromCfg(cfg)
	if err != nil {
		return nil, err
	}

	var httpAddr netip.AddrPort
	if cfg.TunHttpAddr != "" {
		httpAddr, err = netip.ParseAddrPort(cfg.TunHttpAddr)
		if err != nil {
			addr, err := netip.ParseAddr(cfg.TunHttpAddr)
			if err != nil {
				return nil, err
			}
			httpAddr = netip.AddrPortFrom(addr, 80)
		}
	} else {
		for _, addr := range vtun.LocalAddrs() {
			if addr.Is4() && !addr.IsLoopback() {
				httpAddr = netip.AddrPortFrom(addr, 80)
				break
			}
		}
	}

	logger.Printf("starting http server on %s on vtun", httpAddr.String())
	listener, err := vtun.ListenTCP(
		ctx,
		"tcp4",
		httpAddr.String(),
	)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("> %s - %s - %s", r.RemoteAddr, r.URL.String(), r.UserAgent())
		io.WriteString(w, "Hello from userspace TCP!")
	})
	server := &http.Server{Handler: mux}
	wg.Go(func() {
		err := server.Serve(listener)
		if err != nil {
			logger.Printf("http on vtun server sopped: %s", err)
		}
	})
	wg.Go(func() {
		<-ctx.Done()
		server.Close()
	})
	return vtun, nil
}
