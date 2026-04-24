package device

import (
	"context"
	"os"
	"os/exec"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/tun"
	"github.com/asciimoth/tuntap"
	"golang.org/x/sys/unix"
)

const linuxTUNCloneDevice = "/dev/net/tun"

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

	nativeTun, err := createLinuxNativeTUN(name, 1500)
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

func createLinuxNativeTUN(name string, mtu int) (tun.Tun, error) {
	fd, err := unix.Open(linuxTUNCloneDevice, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}

	// Intentionally keep Linux native TUNs in non-VNET_HDR mode.
	// The current forwarding path reads one packet buffer at a time; with
	// VNET_HDR enabled the kernel may deliver large GSO frames that require
	// multi-buffer splitting, which can stall the VPN under heavy traffic.
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}

	file := os.NewFile(uintptr(fd), linuxTUNCloneDevice)
	dev, err := tuntap.CreateTUNFromFile(file, mtu)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return dev, nil
}
