package device

import "net/netip"

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
