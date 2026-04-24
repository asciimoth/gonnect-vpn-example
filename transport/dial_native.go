//go:build !js

package transport

import (
	"fmt"
	"net"
	"net/http"
	"syscall"

	"github.com/asciimoth/gonnect"
	"github.com/coder/websocket"
)

type DialConfig struct {
	Dialer        gonnect.Dial
	ProtectSocket func(fd int) error
}

func dialOptionsFromConfig(cfg DialConfig) (*websocket.DialOptions, error) {
	if cfg.Dialer != nil && cfg.ProtectSocket != nil {
		return nil, fmt.Errorf("custom websocket dialer and socket protection cannot be combined")
	}

	if cfg.Dialer == nil && cfg.ProtectSocket == nil {
		return nil, nil
	}

	transport := &http.Transport{}
	if cfg.Dialer != nil {
		transport.DialContext = cfg.Dialer
	} else if cfg.ProtectSocket != nil {
		transport.DialContext = (&net.Dialer{
			Control: func(_, _ string, rawConn syscall.RawConn) error {
				var protectErr error
				if err := rawConn.Control(func(fd uintptr) {
					protectErr = cfg.ProtectSocket(int(fd))
				}); err != nil {
					return err
				}
				return protectErr
			},
		}).DialContext
	}

	return &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: transport,
		},
	}, nil
}
