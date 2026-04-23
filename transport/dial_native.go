//go:build !js

package transport

import (
	"net/http"

	"github.com/asciimoth/gonnect"
	"github.com/coder/websocket"
)

func dialOptionsFromDialer(dialer gonnect.Dial) (*websocket.DialOptions, error) {
	if dialer == nil {
		return nil, nil
	}

	return &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				DialContext: dialer,
			},
		},
	}, nil
}
