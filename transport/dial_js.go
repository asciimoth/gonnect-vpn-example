//go:build js

package transport

import (
	"fmt"

	"github.com/asciimoth/gonnect"
	"github.com/coder/websocket"
)

func dialOptionsFromDialer(dialer gonnect.Dial) (*websocket.DialOptions, error) {
	if dialer != nil {
		return nil, fmt.Errorf("custom websocket dialers are not supported in js/wasm")
	}
	return nil, nil
}
