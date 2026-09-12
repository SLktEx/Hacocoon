package dnsproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/egress"
	"github.com/SLktEx/Hacocoon/internal/nameresolution"
)

type PlatformResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// Provider uses the Physical Host resolver, including WSL DNS tunneling when
// configured. No public DNS address, guest-selected upstream or Windows exe is used.
type Provider struct{ Resolver PlatformResolver }

func (Provider) Capability() string { return nameresolution.Capability }
func (p Provider) Execute(ctx context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	canonical, err := egress.CanonicalHost(request.Resource)
	if err != nil || canonical != request.Resource || request.Capability != nameresolution.Capability || request.Action != nameresolution.Action || len(request.Attributes) != 0 || len(request.Parameters) != 0 || request.Environment == "" {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, err := resolver.LookupNetIP(ctx, "ip", canonical)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return core.CapabilityResult{}, core.ErrNotFound
		}
		if ctx.Err() != nil {
			return core.CapabilityResult{}, ctx.Err()
		}
		return core.CapabilityResult{}, core.ErrRuntimeUnavailable
	}
	if len(addresses) > nameresolution.MaxAddresses {
		return core.CapabilityResult{}, core.ErrIncompatibleState
	}
	unique := make([]netip.Addr, 0, len(addresses))
	seen := map[netip.Addr]bool{}
	for _, ip := range addresses {
		if !ip.IsValid() || ip.Zone() != "" {
			return core.CapabilityResult{}, core.ErrIncompatibleState
		}
		ip = ip.Unmap()
		if !seen[ip] {
			seen[ip] = true
			unique = append(unique, ip)
		}
	}
	output, err := json.Marshal(unique)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	return core.CapabilityResult{Provider: "standard.dns", Output: string(output)}, nil
}
