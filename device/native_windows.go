//go:build windows

package device

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/tuntap"
)

func nativeFromCfg(
	_ context.Context,
	cfg *cfg.Cfg,
	_ *sync.WaitGroup,
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

	ip, prefix, err := parseIPv4Prefix(addr)
	if err != nil {
		return nil, err
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

	if nt, ok := nativeTun.(*tuntap.NativeTun); ok {
		version, err := nt.RunningVersion()
		if err == nil {
			logger.Printf(
				"wintun driver version: %d.%d.%d.%d",
				(version>>48)&0xffff,
				(version>>32)&0xffff,
				(version>>16)&0xffff,
				version&0xffff,
			)
		}
	}

	cmds := [][]string{
		{
			"interface", "ip", "set", "address",
			fmt.Sprintf("name=%s", actualName),
			"source=static",
			fmt.Sprintf("addr=%s", ip),
			fmt.Sprintf("mask=%s", cidrPrefixToMask(prefix)),
			"gateway=none",
		},
		{
			"interface", "set", "interface",
			fmt.Sprintf("name=%s", actualName),
			"admin=ENABLED",
		},
	}

	ifaceIdx := 0
	if shouldAddNativeRoute(addr, subnet) {
		ifaceIdx, err = getInterfaceIndex(actualName)
		if err != nil {
			_ = nativeTun.Close()
			return nil, err
		}
		cmds = append(cmds, []string{
			"interface", "ipv4", "add", "route",
			subnet,
			fmt.Sprintf("interface=%d", ifaceIdx),
			"store=active",
		})
	} else {
		logger.Printf("skipping explicit route for %s; kernel installs it from %s", subnet, addr)
	}

	for _, cmd := range cmds {
		logger.Printf("running: netsh %v", cmd)
		out, err := exec.Command("netsh", cmd...).CombinedOutput()
		if err != nil {
			logger.Printf("command netsh %v failed: %v\noutput: %s", cmd, err, string(out))
			_ = nativeTun.Close()
			return nil, err
		}
	}

	if ifaceIdx != 0 {
		logger.Printf("route added: %s -> interface %d", subnet, ifaceIdx)
	}
	logger.Printf("interface %s configured with %s", actualName, addr)

	return nativeTun, nil
}

func parseIPv4Prefix(value string) (string, int, error) {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return "", 0, fmt.Errorf("parse tun addr %q: %w", value, err)
	}
	if !prefix.Addr().Is4() {
		return "", 0, fmt.Errorf("native Windows tun requires IPv4 address, got %q", value)
	}
	return prefix.Addr().String(), prefix.Bits(), nil
}

func getInterfaceIndex(name string) (int, error) {
	out, err := exec.Command("netsh", "interface", "ipv4", "show", "interfaces").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("list interfaces: %w\noutput: %s", err, string(out))
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, name) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		idx, err := strconv.Atoi(fields[0])
		if err == nil {
			return idx, nil
		}
	}

	return 0, fmt.Errorf("interface %q not found in netsh output", name)
}

func cidrPrefixToMask(prefix int) string {
	if prefix <= 0 {
		return "0.0.0.0"
	}
	if prefix >= 32 {
		return "255.255.255.255"
	}

	mask := uint32(0xFFFFFFFF) << (32 - prefix)
	return fmt.Sprintf(
		"%d.%d.%d.%d",
		(mask>>24)&0xFF,
		(mask>>16)&0xFF,
		(mask>>8)&0xFF,
		mask&0xFF,
	)
}
