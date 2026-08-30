package environment

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type egressEnvironmentLister interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
}

type sourceIPProvider interface {
	ResolveSourceIP(context.Context, net.IP) (string, error)
}

// EgressSourceResolver maps a transport source address back to one persisted
// managed Environment. The network provider supplies the provider-local runtime
// reference; the persisted Environment store is the authority for whether that
// runtime belongs to Hacocoon and which Environment identity is audited.
type EgressSourceResolver struct {
	router *Router
	store  egressEnvironmentLister
}

func NewEgressSourceResolver(router *Router, store egressEnvironmentLister) (*EgressSourceResolver, error) {
	if router == nil || store == nil {
		return nil, core.ErrInvalidArgument
	}
	return &EgressSourceResolver{router: router, store: store}, nil
}

func (r *EgressSourceResolver) ResolveEnvironment(ctx context.Context, ip net.IP) (string, error) {
	if r == nil || r.router == nil || r.store == nil || ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return "", core.ErrInvalidArgument
	}
	environments, err := r.store.ListEnvironments(ctx)
	if err != nil {
		return "", err
	}
	providerIDs := map[string]struct{}{}
	for _, environment := range environments {
		providerID, _, err := decodeRouteRef(environment.RuntimeRef)
		if err != nil {
			return "", fmt.Errorf("decode persisted runtime ref for Environment %q: %w", environment.Name, core.ErrIncompatibleState)
		}
		providerIDs[providerID] = struct{}{}
	}
	ids := make([]string, 0, len(providerIDs))
	for id := range providerIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	matches := make([]string, 0, 1)
	for _, providerID := range ids {
		provider, err := r.router.provider(providerID)
		if err != nil {
			return "", err
		}
		resolver, ok := provider.(sourceIPProvider)
		if !ok {
			continue
		}
		rawRef, err := resolver.ResolveSourceIP(ctx, ip)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("resolve source IP through provider %q: %w", providerID, err)
		}
		for _, environment := range environments {
			id, ref, err := decodeRouteRef(environment.RuntimeRef)
			if err != nil {
				return "", fmt.Errorf("decode persisted runtime ref for Environment %q: %w", environment.Name, core.ErrIncompatibleState)
			}
			if id == providerID && ref == rawRef {
				matches = append(matches, environment.Name)
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("source IP is not owned by a persisted managed Environment: %w", core.ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("source IP maps to multiple persisted Environments: %w", core.ErrIncompatibleState)
	}
}
