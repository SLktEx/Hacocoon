package nameresolution

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/egress"
)

const Capability = "network.resolve"
const Action = "lookup"
const MaxAddresses = 32

type Requester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}
type Broker struct{ capabilities Requester }

func New(capabilities Requester) *Broker { return &Broker{capabilities: capabilities} }

// Resolve requests address discovery only. Its result is never a connect grant.
func (b *Broker) Resolve(ctx context.Context, environment, host string) ([]netip.Addr, error) {
	return b.resolve(ctx, environment, "", host)
}

// ResolveInstance binds discovery to the source generation already observed by
// a trusted transport. Ordinary Resolve callers retain their existing behavior.
func (b *Broker) ResolveInstance(ctx context.Context, environment, instance, host string) ([]netip.Addr, error) {
	if !core.ValidEnvironmentInstanceID(instance) {
		return nil, core.ErrInvalidArgument
	}
	return b.resolve(ctx, environment, instance, host)
}
func (b *Broker) resolve(ctx context.Context, environment, instance, host string) ([]netip.Addr, error) {
	if b == nil || b.capabilities == nil {
		return nil, core.ErrPolicyDenied
	}
	host, err := egress.CanonicalHost(host)
	if err != nil || strings.TrimSpace(environment) == "" || strings.TrimSpace(environment) != environment || strings.ContainsAny(environment, "\x00\r\n") {
		return nil, core.ErrInvalidArgument
	}
	result, err := b.capabilities.Request(ctx, core.CapabilityRequest{Capability: Capability, Action: Action, Environment: environment, EnvironmentInstance: instance, Resource: host})
	if err != nil {
		return nil, err
	}
	if !result.AuditComplete || result.ExecutionState != core.CapabilitySucceeded || len(result.Output) > 4096 {
		return nil, core.ErrIncompatibleState
	}
	var addresses []netip.Addr
	if json.Unmarshal([]byte(result.Output), &addresses) != nil || len(addresses) > MaxAddresses {
		return nil, core.ErrIncompatibleState
	}
	for _, address := range addresses {
		if !address.IsValid() || address.Zone() != "" {
			return nil, core.ErrIncompatibleState
		}
	}
	return addresses, nil
}
