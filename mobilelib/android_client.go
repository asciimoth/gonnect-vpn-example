package mobilelib

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/clientcore"
)

type SocketProtector interface {
	Protect(fd int32) bool
}

type AndroidVPNClient struct {
	mu      sync.Mutex
	session *clientcore.AndroidTunClientSession
	status  string
	logs    []string
}

func NewAndroidVPNClient() *AndroidVPNClient {
	c := &AndroidVPNClient{
		status: "idle",
	}
	c.appendLogLocked("android vpn client ready")
	return c
}

func (c *AndroidVPNClient) Connect(connectURL string, tunFD int32, tunName string, protector SocketProtector) error {
	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return fmt.Errorf("android vpn client is already connected")
	}
	c.status = "connecting"
	c.appendLogLocked("android vpn connect requested")
	c.mu.Unlock()

	if strings.TrimSpace(tunName) == "" {
		tunName = defaultTunName
	}

	var protectFunc func(fd int) error
	if protector != nil {
		protectFunc = func(fd int) error {
			if protector.Protect(int32(fd)) {
				return nil
			}
			return fmt.Errorf("vpn service failed to protect socket fd %d", fd)
		}
	}

	session, err := clientcore.NewAndroidTunClientSession(context.Background(), &cfg.Cfg{
		Connect: strings.TrimSpace(connectURL),
		TunName: tunName,
	}, int(tunFD), protectFunc, androidClientLogger{client: c})
	if err != nil {
		c.setStatusf("connect failed: %v", err)
		return err
	}

	c.mu.Lock()
	c.session = session
	c.status = "connected"
	c.appendLogLocked("android vpn connected")
	c.mu.Unlock()
	return nil
}

func (c *AndroidVPNClient) Disconnect() {
	session := c.takeSession("disconnecting")
	if session == nil {
		c.setStatus("idle")
		return
	}

	session.Stop()
	session.Wait()
	c.mu.Lock()
	c.status = "idle"
	c.appendLogLocked("android vpn disconnected")
	c.mu.Unlock()
}

func (c *AndroidVPNClient) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *AndroidVPNClient) Logs() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.logs, "\n")
}

func (c *AndroidVPNClient) takeSession(nextStatus string) *clientcore.AndroidTunClientSession {
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

func (c *AndroidVPNClient) setStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *AndroidVPNClient) setStatusf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = fmt.Sprintf(format, args...)
	c.appendLogLocked(c.status)
}

func (c *AndroidVPNClient) appendLogLocked(line string) {
	appendLogLocked(&c.logs, line)
}

type androidClientLogger struct {
	client *AndroidVPNClient
}

func (l androidClientLogger) Print(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l androidClientLogger) Println(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l androidClientLogger) Printf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l androidClientLogger) Fatal(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l androidClientLogger) Fatalf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l androidClientLogger) Fatalln(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l androidClientLogger) Panic(v ...any) {
	l.log(fmt.Sprint(v...))
}

func (l androidClientLogger) Panicf(format string, v ...any) {
	l.log(fmt.Sprintf(format, v...))
}

func (l androidClientLogger) Panicln(v ...any) {
	l.log(fmt.Sprintln(v...))
}

func (l androidClientLogger) log(line string) {
	if l.client == nil {
		return
	}
	l.client.mu.Lock()
	defer l.client.mu.Unlock()
	l.client.appendLogLocked(line)
}
