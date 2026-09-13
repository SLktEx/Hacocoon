package egress

import "github.com/SLktEx/Hacocoon/internal/core"

// DefaultEndpoint is one narrowly scoped destination that Hacocoon may grant
// before the operator Policy default is consulted. It is authority metadata,
// not a guest-discovered network configuration value.
type DefaultEndpoint struct {
	Host     string
	Protocol core.EgressProtocol
	Port     int
}

var officialUbuntuPackageRepositoryHosts = [...]string{
	"archive.ubuntu.com",
	"security.ubuntu.com",
	"ports.ubuntu.com",
}

// DefaultPackageRepositoryEndpoints returns the fixed package-repository
// destinations used by Hacocoon's official Ubuntu Bases. The returned slice is
// rebuilt on every call so callers cannot mutate the product source of truth.
//
// Guest APT configuration is deliberately not inspected here: adding or
// replacing /etc/apt/sources.list entries must never expand network authority.
func DefaultPackageRepositoryEndpoints() []DefaultEndpoint {
	endpoints := make([]DefaultEndpoint, 0, len(officialUbuntuPackageRepositoryHosts)*2)
	for _, host := range officialUbuntuPackageRepositoryHosts {
		endpoints = append(endpoints,
			DefaultEndpoint{Host: host, Protocol: core.EgressHTTP, Port: 80},
			DefaultEndpoint{Host: host, Protocol: core.EgressHTTPS, Port: 443},
		)
	}
	return endpoints
}
