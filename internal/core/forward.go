package core

import "net/netip"

// EnvironmentTCPForward pins a management-side listener to one creation. It
// carries no provider route, credential or reusable guest capability.
type EnvironmentTCPForward struct {
	Environment string `json:"environment"`
	Instance    string `json:"instance"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
}

func ValidForwardAddress(address string, port int) bool {
	ip, err := netip.ParseAddr(address)
	return err == nil && ip.Zone() == "" && !ip.Is4In6() && ip.IsLoopback() && port >= 1 && port <= 65535
}
