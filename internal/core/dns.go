package core

import (
	"context"
	"net/netip"
)

// DNSMode selects name discovery only. It never grants a connection capability.
type DNSMode string

const (
	DNSHost     DNSMode = "host"
	DNSBackend  DNSMode = "backend"
	DNSDisabled DNSMode = "disabled"
)

// Effective makes the ordinary Host resolver the explicit default.
func (m DNSMode) Effective() DNSMode {
	if m == "" {
		return DNSHost
	}
	return m
}

func (m DNSMode) Valid() bool {
	switch m.Effective() {
	case DNSHost, DNSBackend, DNSDisabled:
		return true
	default:
		return false
	}
}

// BackendNameResolver selects a provider-owned resolver for one exact instance.
// Neither its answers nor this interface confer outbound connection authority.
type BackendNameResolver interface {
	ResolveEnvironmentName(context.Context, string, string, string) ([]netip.Addr, error)
}
