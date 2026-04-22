//go:build windows

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
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
) (tun.Tun, error) {
	return nil, fmt.Errorf("native tun type is not supported on this platform")
}
