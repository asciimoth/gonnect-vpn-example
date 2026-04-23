//go:build unix

package device

import (
	"context"
	"net/netip"
	"os/exec"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/tuntap"
)

func nativeFromCfg(
	ctx context.Context,
	cfg *cfg.Cfg,
	wg *sync.WaitGroup,
	logger logger.Logger,
) (tun.Tun, error) {
	name := "tun0"
	if cfg.TunName != "" {
		name = cfg.TunName
	}
	addr := defaultNativeAddr
	if cfg.TunAddr != "" {
		addr = cfg.TunAddr
	}
	subnet := defaultNativeSubnet
	if cfg.TunSubnet != "" {
		subnet = cfg.TunSubnet
	}

	nativeTun, err := tuntap.CreateTUN(name, 1500)
	if err != nil {
		return nil, err
	}

	actualName, err := nativeTun.Name()
	if err != nil {
		logger.Printf("warning: could not get interface name: %v", err)
		actualName = name
	}

	logger.Printf("created native TUN device: %q", actualName)

	// Waiting for UP
	<-nativeTun.Events()

	cmds := [][]string{
		{"ip", "link", "set", "dev", actualName, "up"},
		{"ip", "-4", "addr", "add", addr, "dev", actualName},
	}
	if shouldAddNativeRoute(addr, subnet) {
		cmds = append(cmds, []string{"ip", "-4", "route", "add", subnet, "dev", actualName})
	} else {
		logger.Printf("skipping explicit route for %s; kernel installs it from %s", subnet, addr)
	}

	for _, cmd := range cmds {
		logger.Printf("running: %v", cmd)
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			logger.Panicln("command %v failed: %v\noutput: %s", cmd, err, string(out))
			_ = nativeTun.Close()
			return nil, err
		}
	}

	logger.Printf("interface %s configured with %s", actualName, addr)

	return nativeTun, nil
}

func shouldAddNativeRoute(addr, subnet string) bool {
	addrPrefix, err := netip.ParsePrefix(addr)
	if err != nil {
		return true
	}
	subnetPrefix, err := netip.ParsePrefix(subnet)
	if err != nil {
		return true
	}
	return addrPrefix.Masked() != subnetPrefix.Masked()
}
