//go:build !unix && !windows

package device

import (
	"context"
	"fmt"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
)

func nativeFromCfg(
	_ context.Context,
	_ *cfg.Cfg,
	_ *sync.WaitGroup,
	_ logger.Logger,
) (tun.Tun, error) {
	return nil, fmt.Errorf("native tun type is not supported on this platform")
}
