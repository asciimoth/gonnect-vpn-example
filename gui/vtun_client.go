package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asciimoth/gonnect-netstack/vtun"
	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/transport"
	"github.com/asciimoth/gonnect/tun"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const defaultGUIVTunAddr = "10.200.1.5"

type sessionHandle interface {
	Stop()
	Wait()
}

type vtunClientSession struct {
	logger logger.Logger

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	conn   *transport.Conn
	vtun   *vtun.VTun
	p2p    *tun.Point2Point
	client *http.Client

	pingSeq atomic.Uint32
}

func newVTunClientSession(
	parent context.Context,
	conf *cfg.Cfg,
	log logger.Logger,
) (*vtunClientSession, error) {
	if conf == nil {
		return nil, fmt.Errorf("config is required")
	}
	if strings.TrimSpace(conf.Connect) == "" {
		return nil, fmt.Errorf("connect url is required")
	}

	addrText := strings.TrimSpace(conf.TunAddr)
	if addrText == "" {
		addrText = defaultGUIVTunAddr
	}
	addr, err := netip.ParseAddr(addrText)
	if err != nil {
		return nil, fmt.Errorf("invalid vtun address %q: %w", addrText, err)
	}

	name := strings.TrimSpace(conf.TunName)
	if name == "" {
		name = "gui-vtun"
	}

	ctx, cancel := context.WithCancel(parent)
	conn, err := transport.Dial(ctx, conf.Connect, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to %s: %w", conf.Connect, err)
	}

	dev, err := (&vtun.Opts{
		LocalAddrs: []netip.Addr{addr},
		Name:       name,
	}).Build()
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to create vtun %q at %s: %w", name, addr, err)
	}

	p2p := tun.NewP2P(nil)
	p2p.SetA(conn)
	p2p.SetB(dev)

	session := &vtunClientSession{
		logger: log,
		cancel: cancel,
		done:   make(chan struct{}),
		conn:   conn,
		vtun:   dev,
		p2p:    p2p,
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: dev.Dial,
			},
		},
	}

	log.Printf("connected to %s with vtun address %s", conf.Connect, addr)

	go func() {
		<-ctx.Done()
		session.closeResources()
		close(session.done)
	}()

	return session, nil
}

func (s *vtunClientSession) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.cancel()
	})
}

func (s *vtunClientSession) Wait() {
	if s == nil {
		return
	}
	<-s.done
}

func (s *vtunClientSession) closeResources() {
	if s.p2p != nil {
		s.p2p.Stop()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.vtun != nil {
		_ = s.vtun.Close()
	}
}

func (s *vtunClientSession) DoRequest(
	ctx context.Context,
	method string,
	targetURL string,
	headersText string,
	bodyText string,
) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("vtun client is not connected")
	}

	method = strings.TrimSpace(method)
	if method == "" {
		method = http.MethodGet
	}
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return "", fmt.Errorf("target url is required")
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(bodyText))
	if err != nil {
		return "", err
	}

	headers, err := parseHeaderLines(headersText)
	if err != nil {
		return "", err
	}
	req.Header = headers

	s.logger.Printf("vtun request started: %s %s", method, targetURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	s.logger.Printf(
		"vtun request finished: %s %s -> %s (%d bytes)",
		method,
		targetURL,
		resp.Status,
		len(body),
	)
	return formatHTTPResponse(resp, body), nil
}

func parseHeaderLines(text string) (http.Header, error) {
	headers := make(http.Header)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("invalid header line %q", line)
		}
		headers.Add(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	return headers, nil
}

func formatHTTPResponse(resp *http.Response, body []byte) string {
	var result bytes.Buffer
	fmt.Fprintf(&result, "%s\n", resp.Status)
	_ = resp.Header.Write(&result)
	result.WriteString("\n")
	result.Write(body)
	return result.String()
}

func (s *vtunClientSession) Ping(ctx context.Context, target string) (string, error) {
	if s == nil || s.vtun == nil {
		return "", fmt.Errorf("vtun client is not connected")
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(target))
	if err != nil {
		return "", fmt.Errorf("invalid ping target %q: %w", target, err)
	}

	conn, err := s.vtun.DialPingAddr(netip.Addr{}, addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	seq := int(s.pingSeq.Add(1))
	id := os.Getpid() & 0xffff
	payload := []byte("gonnect-gui-ping")

	proto := 1
	var msgType icmp.Type = ipv4.ICMPTypeEcho
	var replyType icmp.Type = ipv4.ICMPTypeEchoReply
	if addr.Is6() {
		proto = 58
		msgType = ipv6.ICMPTypeEchoRequest
		replyType = ipv6.ICMPTypeEchoReply
	}

	msg := icmp.Message{
		Type: msgType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: payload,
		},
	}
	packet, err := msg.Marshal(nil)
	if err != nil {
		return "", err
	}

	deadline := time.Now().Add(3 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", err
	}

	start := time.Now()
	if _, err := conn.Write(packet); err != nil {
		return "", err
	}

	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return "", err
		}

		reply, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			return "", err
		}
		if reply.Type != replyType {
			s.logger.Printf("vtun ping ignored icmp packet from %s: type=%v", addr, reply.Type)
			continue
		}

		echo, ok := reply.Body.(*icmp.Echo)
		if !ok {
			s.logger.Printf("vtun ping ignored icmp packet from %s: body=%T", addr, reply.Body)
			continue
		}
		if echo.ID != id || echo.Seq != seq {
			s.logger.Printf(
				"vtun ping ignored mismatched echo reply from %s: id=%d seq=%d expected_id=%d expected_seq=%d",
				addr,
				echo.ID,
				echo.Seq,
				id,
				seq,
			)
			continue
		}

		rtt := time.Since(start).Round(time.Millisecond)
		s.logger.Printf("vtun ping reply: %s seq=%d time=%s", addr, seq, rtt)
		return fmt.Sprintf("Reply from %s: seq=%d bytes=%d time=%s", addr, seq, n, rtt), nil
	}
}
