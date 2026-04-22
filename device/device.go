package device

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"sync"

	"github.com/asciimoth/gonnect-netstack/vtun"
	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
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

func vtunFromCfg(cfg *cfg.Cfg) (*vtun.VTun, error) {
	laddrs := []netip.Addr{
		netip.MustParseAddr("10.200.2.1"),
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
	go func() {
		<-ctx.Done()
		server.Close()
	}()
	return vtun, nil
}
