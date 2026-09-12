package networkrelay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type Catalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	EnvironmentInstance(context.Context, core.Environment) (string, error)
}
type Configuration interface {
	Snapshot(context.Context) (capability.PolicySnapshot, error)
}
type Evaluator interface {
	Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error)
}
type Auditor interface {
	Record(context.Context, core.CapabilityAuditEvent) error
}
type Resolver interface {
	ResolveInstance(context.Context, string, string, string) ([]netip.Addr, error)
}

type ConfiguredAuthority struct {
	Catalog       Catalog
	Configuration Configuration
	Policy        Evaluator
	Audit         Auditor
}

func (a *ConfiguredAuthority) CurrentInstance(ctx context.Context, name string) (string, error) {
	env, err := a.Catalog.GetEnvironment(ctx, name)
	if err != nil {
		return "", err
	}
	return a.Catalog.EnvironmentInstance(ctx, env)
}
func (a *ConfiguredAuthority) PolicyRevision(ctx context.Context) (string, error) {
	snapshot, err := a.Configuration.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	var policy capability.PolicyFile
	if json.Unmarshal(snapshot.Policy, &policy) != nil {
		return "", core.ErrPolicyDenied
	}
	// Expiration changes authority without requiring a file write. Include every
	// rule's active/expired phase; unrelated edits conservatively close sessions.
	h := sha256.New()
	h.Write([]byte(snapshot.Revision))
	now := time.Now()
	for _, r := range append(policy.Rules, policy.SavedDecisions...) {
		if r.ExpiresAt != nil && !now.Before(*r.ExpiresAt) {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (a *ConfiguredAuthority) Evaluate(ctx context.Context, r core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return a.Policy.Evaluate(ctx, r)
}
func (a *ConfiguredAuthority) Record(ctx context.Context, e core.CapabilityAuditEvent) error {
	return a.Audit.Record(ctx, e)
}

type ConfiguredTargets struct {
	Configuration Configuration
	Authority     Authority
	DNS           Resolver
	// ExternalAllowed must also reject Host and managed-network addresses;
	// selecting an external address must never bypass the named target boundary.
	HostAllowed     func(context.Context, netip.Addr) error
	ExternalAllowed func(context.Context, netip.Addr) error
}

func (t *ConfiguredTargets) Resolve(ctx context.Context, source Source, spec Spec) (Target, error) {
	target := Target{Kind: spec.Kind, Name: spec.Target, Port: spec.Port, Protocol: spec.Protocol}
	switch spec.Kind {
	case "host":
		service, err := t.host(ctx, spec.Target)
		if err != nil {
			return Target{}, err
		}
		if service.Protocol != spec.Protocol || (spec.Port != 0 && service.Port != spec.Port) {
			return Target{}, core.ErrPolicyDenied
		}
		target.Owner = service.Instance
		target.Port = service.Port
		target.Addresses = []netip.Addr{netip.MustParseAddr(service.Address)}
	case "environment":
		instance, err := t.Authority.CurrentInstance(ctx, spec.Target)
		if err != nil {
			return Target{}, core.ErrCapabilityStale
		}
		target.Owner = instance
		// The provider creates this socket inside the pinned destination network
		// namespace. This is not the controller's loopback.
		target.Addresses = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	case "external":
		address, err := netip.ParseAddr(spec.Target)
		if err == nil {
			target.Addresses = []netip.Addr{address}
		} else {
			target.Addresses, err = t.DNS.ResolveInstance(ctx, source.Environment, source.Instance, spec.Target)
			if err != nil {
				if errors.Is(err, core.ErrPolicyDenied) || errors.Is(err, core.ErrApprovalDenied) || errors.Is(err, core.ErrCapabilityStale) {
					return Target{}, err
				}
				return Target{}, errDNSResolution
			}
		}
	default:
		return Target{}, core.ErrInvalidArgument
	}
	if err := t.Verify(ctx, target); err != nil {
		return Target{}, err
	}
	return target, nil
}
func (t *ConfiguredTargets) Verify(ctx context.Context, target Target) error {
	switch target.Kind {
	case "environment":
		instance, err := t.Authority.CurrentInstance(ctx, target.Name)
		if err != nil || instance != target.Owner {
			return core.ErrCapabilityStale
		}
	case "host":
		s, err := t.host(ctx, target.Name)
		if err != nil || s.Instance != target.Owner || s.Protocol != target.Protocol || s.Port != target.Port ||
			len(target.Addresses) != 1 || s.Address != target.Addresses[0].String() {
			return core.ErrCapabilityStale
		}
		if t.HostAllowed == nil {
			return core.ErrPolicyDenied
		}
		if err := t.HostAllowed(ctx, target.Addresses[0]); err != nil {
			return err
		}
		// Standard control/egress and commonly exposed privileged daemon ports
		// are not development-service registrations.
		switch s.Port {
		case 18080, 8443, 2375, 2376:
			return core.ErrPolicyDenied
		}
	case "external":
		if t.ExternalAllowed == nil || len(target.Addresses) == 0 {
			return core.ErrPolicyDenied
		}
		for _, a := range target.Addresses {
			if err := t.ExternalAllowed(ctx, a); err != nil {
				return err
			}
		}
	default:
		return core.ErrInvalidArgument
	}
	return nil
}
func (t *ConfiguredTargets) host(ctx context.Context, name string) (capability.NetworkService, error) {
	snapshot, err := t.Configuration.Snapshot(ctx)
	if err != nil {
		return capability.NetworkService{}, err
	}
	var policy capability.PolicyFile
	if json.Unmarshal(snapshot.Policy, &policy) != nil {
		return capability.NetworkService{}, core.ErrPolicyDenied
	}
	for _, service := range policy.NetworkServices {
		if service.Name == name {
			if capability.ValidateNetworkService(service) != nil {
				return capability.NetworkService{}, core.ErrPolicyDenied
			}
			return service, nil
		}
	}
	return capability.NetworkService{}, core.ErrNotFound
}
