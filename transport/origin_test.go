package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/coder/websocket"
)

func TestAcceptAllowsCrossOrigin(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := transport.Accept(ctx, w, r)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer conn.Close() // nolint:errcheck
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	ws, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"https://asciimoth.github.io"},
		},
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close() // nolint:errcheck
	}
	if err != nil {
		t.Fatalf("dial websocket with foreign origin: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "") // nolint:errcheck
}
