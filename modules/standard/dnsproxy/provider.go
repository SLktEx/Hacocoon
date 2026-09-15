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

// EnvironmentCatalog binds resolver selection to the current ready creation.
type EnvironmentCatalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	EnvironmentInstance(context.Context, core.Environment) (string, error)
}
// Provider selects the current Environment resolver after Policy and audit.
// Host mode uses the Physical Host; backend details remain behind its seam.
type Provider struct {
	Resolver     PlatformResolver
	Environments EnvironmentCatalog
	Backend      core.BackendNameResolver
}

func (Provider) Capability() string { return nameresolution.Capability }
func (p Provider) Execute(ctx context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	canonical, err := egress.CanonicalHost(request.Resource)
	if err != nil || canonical != request.Resource || request.Capability != nameresolution.Capability || request.Action != nameresolution.Action || len(request.Attributes) != 0 || len(request.Parameters) != 0 || request.Environment == "" {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	if p.Environments == nil {
		return core.CapabilityResult{}, core.ErrRuntimeUnavailable
	}
	environment, err := p.Environments.GetEnvironment(ctx, request.Environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if environment.Name != request.Environment || !environment.DNSMode.Valid() {
		return core.CapabilityResult{}, core.ErrIncompatibleState
	}
	instance, err := p.Environments.EnvironmentInstance(ctx, environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if !core.ValidEnvironmentInstanceID(instance) {
		return core.CapabilityResult{}, core.ErrIncompatibleState
	}
	if request.EnvironmentInstance != "" && request.EnvironmentInstance != instance {
		return core.CapabilityResult{}, core.ErrCapabilityStale
	}
	if environment.DNSMode.Effective() == core.DNSDisabled {
		return core.CapabilityResult{}, core.ErrPolicyDenied
	}
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var addresses []netip.Addr
	if environment.DNSMode.Effective() == core.DNSBackend {
		if p.Backend == nil {
			return core.CapabilityResult{}, core.ErrUnsupported
		}
		addresses, err = p.Backend.ResolveEnvironmentName(ctx, environment.RuntimeRef, instance, canonical)
	} else {
		addresses, err = resolver.LookupNetIP(ctx, "ip", canonical)
	}
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
	current, checkErr := p.Environments.EnvironmentInstance(ctx, environment)
	if checkErr != nil || current != instance {
		return core.CapabilityResult{}, core.ErrCapabilityStale
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
