//go:build android

package clientcore

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/tuntap"
)

type AndroidTunClientSession struct {
	logger logger.Logger

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	conn *transport.Conn
	tun  tun.Tun
	p2p  *tun.Point2Point
}

func NewAndroidTunClientSession(
	parent context.Context,
	conf *cfg.Cfg,
	tunFD int,
	protectSocket func(fd int) error,
	log logger.Logger,
) (*AndroidTunClientSession, error) {
	if conf == nil {
		return nil, fmt.Errorf("config is required")
	}
	if strings.TrimSpace(conf.Connect) == "" {
		return nil, fmt.Errorf("connect url is required")
	}
	if tunFD < 0 {
		return nil, fmt.Errorf("tun fd is required")
	}

	ctx, cancel := context.WithCancel(parent)

	dev, name, err := tuntap.CreateUnmonitoredTUNFromFD(tunFD)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create android tun from fd %d: %w", tunFD, err)
	}

	conn, err := transport.DialWithConfig(ctx, conf.Connect, transport.DialConfig{
		ProtectSocket: protectSocket,
	})
	if err != nil {
		cancel()
		_ = dev.Close()
		return nil, fmt.Errorf("failed to connect to %s: %w", conf.Connect, err)
	}

	p2p := tun.NewP2P(nil)
	p2p.SetA(conn)
	p2p.SetB(dev)

	session := &AndroidTunClientSession{
		logger: log,
		cancel: cancel,
		done:   make(chan struct{}),
		conn:   conn,
		tun:    dev,
		p2p:    p2p,
	}

	log.Printf("connected to %s with android tun %s", conf.Connect, name)

	go func() {
		<-ctx.Done()
		session.closeResources()
		close(session.done)
	}()

	return session, nil
}

func (s *AndroidTunClientSession) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.cancel()
	})
}

func (s *AndroidTunClientSession) Wait() {
	if s == nil {
		return
	}
	<-s.done
}

func (s *AndroidTunClientSession) closeResources() {
	if s.p2p != nil {
		s.p2p.Stop()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.tun != nil {
		_ = s.tun.Close()
	}
}
