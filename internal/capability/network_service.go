package capability

import (
	"net/netip"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// NetworkService is an administrator-selected TCP/UDP endpoint, stored and
// audited with Policy configuration. It contains no authentication material.
type NetworkService struct {
	Name     string `json:"name"`
	Instance string `json:"instance"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
}

var networkServiceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var networkServiceInstance = regexp.MustCompile(`^svc-[a-f0-9]{32}$`)

func ValidateNetworkService(s NetworkService) error {
	address, err := netip.ParseAddr(s.Address)
	if !networkServiceName.MatchString(s.Name) || !networkServiceInstance.MatchString(s.Instance) ||
		(s.Protocol != "tcp" && s.Protocol != "udp") || s.Port < 1 || s.Port > 65535 ||
		err != nil || address.String() != s.Address || address.Is4In6() || address.Zone() != "" ||
		address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() {
		return core.ErrInvalidArgument
	}
	return nil
}
