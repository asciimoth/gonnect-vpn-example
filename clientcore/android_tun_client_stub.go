//go:build !android

package clientcore

import (
	"context"
	"fmt"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
)

type AndroidTunClientSession struct{}

func NewAndroidTunClientSession(
	_ context.Context,
	_ *cfg.Cfg,
	_ int,
	_ func(int) error,
	_ logger.Logger,
) (*AndroidTunClientSession, error) {
	return nil, fmt.Errorf("android tun client session is only supported on android")
}

func (s *AndroidTunClientSession) Stop() {}

func (s *AndroidTunClientSession) Wait() {}
