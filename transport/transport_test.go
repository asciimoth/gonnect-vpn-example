// nolint
package transport_test

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/asciimoth/gonnect"
	"github.com/asciimoth/gonnect-netstack/vtun"

	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect/loopback"
	gt "github.com/asciimoth/gonnect/testing"
	"github.com/asciimoth/gonnect/tun"
)

// TCP client <-> vtun <-> WS client <-> loopback <-> WS server <-> vtun <-> TCP server
func TestTransportTCPingPong(t *testing.T) {
	log := log.New(os.Stdout, t.Name()+" ", 0)

	optsServer := vtun.Opts{
		Name: "server tun",
	}
	tunServer, err := optsServer.Build()
	if err != nil {
		panic(err)
	}
	defer tunServer.Close() // nolint
	optsClient := vtun.Opts{
		Name: "client tun",
	}
	tunClient, err := optsClient.Build()
	if err != nil {
		panic(err)
	}
	defer tunClient.Close() // nolint

	// TODO: Use auto addr allocation

	// Wait for both tunnels to be up
	<-tunClient.Events()
	<-tunServer.Events()

	loop := loopback.NewLoopbackNetwok()
	defer loop.Down() // nolint

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	var swg sync.WaitGroup

	wg.Go(func() {
		server(ctx, loop, &swg, log, tunServer)
	})

	time.Sleep(time.Millisecond * 200)

	tc, err := transport.Dial(ctx, "ws://127.0.0.1:80", loop.Dial)
	if err != nil {
		t.Fatal("transport connect", err)
	}
	defer tc.Close() //nolint
	log.Print("Transport client connected")

	p2pClient := tun.NewP2P(nil)
	defer p2pClient.Stop()
	p2pClient.SetA(&tun.CallbackTUN{
		Tun: tunClient,
		OnWrite: func(n int, err error) {
			if err != nil {
				return
			}
			log.Println("vtun --IP->")
		},
		OnRead: func(n int, err error) {
			if err != nil {
				return
			}
			log.Println("vtun <-IP--")
		},
	})
	p2pClient.SetB(&tun.CallbackTUN{
		Tun: tc,
		OnWrite: func(n int, err error) {
			if err != nil {
				return
			}
			log.Println("--IP-> transport")
		},
		OnRead: func(n int, err error) {
			if err != nil {
				return
			}
			log.Println("<-IP-- transport")
		},
	})

	time.Sleep(time.Millisecond * 200)

	listener, err := tunServer.Listen(ctx, "tcp4", "0.0.0.0:1234")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	gt.RunTCPPingPongTest(t, listener, func(addr net.Addr) (net.Conn, error) {
		conn, err := tunClient.Dial(ctx, addr.Network(), addr.String())
		log.Println("Vtun Dial", conn, err)
		return conn, err
	})

	log.Print("Shutdown")

	_ = listener.Close()
	cancel()
	swg.Wait()
	_ = loop.Down()
	_ = tunServer.Close()
	_ = tunClient.Close()
	wg.Wait()
}

func server(
	ctx context.Context,
	loop gonnect.Network,
	wg *sync.WaitGroup,
	log logger.Logger,
	t tun.Tun,
) {
	listener, err := loop.ListenTCP(
		context.Background(),
		"tcp4",
		"0.0.0.0:80",
	)
	if err != nil {
		panic(err)
	}
	log.Println("Server listener started")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Accepted http conn")
		conn, err := transport.Accept(ctx, w, r)
		if err != nil {
			panic(err)
		}
		log.Println("Accepted ws conn")
		p2pServer := tun.NewP2P(nil)
		p2pServer.SetA(t)
		p2pServer.SetB(conn)
		wg.Go(func() {
			defer conn.Close() // nolint
			defer p2pServer.Stop()
			<-ctx.Done()
		})
	})
	server := &http.Server{
		Addr:    "0.0.0.0:80",
		Handler: mux,
	}
	wg.Go(func() {
		<-ctx.Done()
		_ = server.Shutdown(ctx)
	})
	log.Println("Starting http server")
	if err := server.Serve(
		listener,
	); err != nil &&
		err.Error() != "http: Server closed" {
		panic(err)
	}
}
