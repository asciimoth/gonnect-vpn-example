//go:build js

package transport

import (
	"fmt"

	"github.com/asciimoth/gonnect"
	"github.com/coder/websocket"
)

type DialConfig struct {
	Dialer        gonnect.Dial
	ProtectSocket func(fd int) error
}

func dialOptionsFromConfig(cfg DialConfig) (*websocket.DialOptions, error) {
	if cfg.Dialer != nil {
		return nil, fmt.Errorf("custom websocket dialers are not supported in js/wasm")
	}
	if cfg.ProtectSocket != nil {
		return nil, fmt.Errorf("socket protection is not supported in js/wasm")
	}
	return nil, nil
}
