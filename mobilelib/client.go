package mobilelib

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/clientcore"
)

const (
	defaultTunName       = "android-vtun"
	defaultRequestTimout = 15 * time.Second
	defaultPingTimeout   = 5 * time.Second
	maxLogLines          = 200
)

type Client struct {
	mu      sync.Mutex
	session *clientcore.VTunClientSession
	status  string
	logs    []string
}

func NewClient() *Client {
	c := &Client{
		status: "idle",
	}
	c.appendLogLocked("client ready")
	return c
}

func (c *Client) Connect(connectURL, tunAddr, tunName string) error {
	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return fmt.Errorf("client is already connected")
	}
	c.status = "connecting"
	c.appendLogLocked("connect requested")
	c.mu.Unlock()

	if strings.TrimSpace(tunName) == "" {
		tunName = defaultTunName
	}

	session, err := clientcore.NewVTunClientSession(context.Background(), &cfg.Cfg{
		Connect: strings.TrimSpace(connectURL),
		TunAddr: strings.TrimSpace(tunAddr),
		TunName: tunName,
	}, clientLogger{client: c})
	if err != nil {
		c.setStatusf("connect failed: %v", err)
		return err
	}

	c.mu.Lock()
	c.session = session
	c.status = "connected"
	c.appendLogLocked("connected")
	c.mu.Unlock()
	return nil
}

func (c *Client) Disconnect() {
	session := c.takeSession("disconnecting")
	if session == nil {
		c.setStatus("idle")
		return
	}

	session.Stop()
	session.Wait()
	c.mu.Lock()
	c.status = "idle"
	c.appendLogLocked("disconnected")
	c.mu.Unlock()
}

func (c *Client) Request(method, targetURL, headersText, bodyText string) (string, error) {
	session := c.currentSession()
	if session == nil {
		return "", fmt.Errorf("client is not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimout)
	defer cancel()

	result, err := session.DoRequest(ctx, method, targetURL, headersText, bodyText)
	if err != nil {
		c.setStatusf("request failed: %v", err)
		return "", err
	}

	c.setStatus("connected")
	return result, nil
}

func (c *Client) Ping(target string) (string, error) {
	session := c.currentSession()
	if session == nil {
		return "", fmt.Errorf("client is not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
	defer cancel()

	result, err := session.Ping(ctx, target)
	if err != nil {
		c.setStatusf("ping failed: %v", err)
		return "", err
	}

	c.setStatus("connected")
	return result, nil
}

func (c *Client) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Client) Logs() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.logs, "\n")
}

func (c *Client) currentSession() *clientcore.VTunClientSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

func (c *Client) takeSession(nextStatus string) *clientcore.VTunClientSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	session := c.session
	c.session = nil
	c.status = nextStatus
	if session != nil {
		c.appendLogLocked(nextStatus)
	}
	return session
}

func (c *Client) setStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *Client) setStatusf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = fmt.Sprintf(format, args...)
	c.appendLogLocked(c.status)
}

func (c *Client) appendLogLocked(line string) {
	appendLogLocked(&c.logs, line)
}

type clientLogger struct {
	client *Client
}

func (l clientLogger) Print(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l clientLogger) Println(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l clientLogger) Printf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l clientLogger) Fatal(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l clientLogger) Fatalf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l clientLogger) Fatalln(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l clientLogger) Panic(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l clientLogger) Panicf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l clientLogger) Panicln(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l clientLogger) log(line string) {
	if l.client == nil {
		return
	}
	l.client.mu.Lock()
	defer l.client.mu.Unlock()
	l.client.appendLogLocked(line)
}

func appendLogLocked(logs *[]string, line string) {
	line = strings.TrimSpace(line)
	if line == "" || logs == nil {
		return
	}

	*logs = append(*logs, line)
	if len(*logs) > maxLogLines {
		*logs = append([]string(nil), (*logs)[len(*logs)-maxLogLines:]...)
	}
}
