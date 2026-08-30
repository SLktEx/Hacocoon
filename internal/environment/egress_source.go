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

// EgressSourceResolver binds transport identity from a provider to persisted
// Hacocoon Environment identity. A provider proves which provider-local runtime
// owns the source IP; persisted state proves that runtime is a managed
// Environment allowed to acquire Environment-scoped policy authority.
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

func (r *EgressSourceResolver) ResolveEnvironment(ctx context.Context, source net.IP) (string, error) {
	if r == nil || r.router == nil || r.store == nil || source == nil || source.IsUnspecified() || source.IsLoopback() || source.IsMulticast() {
		return "", core.ErrPolicyDenied
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
		rawRef, err := resolver.ResolveSourceIP(ctx, source)
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrPolicyDenied) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("resolve source IP through provider %q: %w", providerID, err)
		}
		for _, environment := range environments {
			id, persistedRef, err := decodeRouteRef(environment.RuntimeRef)
			if err != nil {
				return "", fmt.Errorf("decode persisted runtime ref for Environment %q: %w", environment.Name, core.ErrIncompatibleState)
			}
			if id == providerID && persistedRef == rawRef {
				matches = append(matches, environment.Name)
			}
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", core.ErrPolicyDenied
	default:
		return "", fmt.Errorf("source IP maps to multiple persisted Environments: %w", core.ErrIncompatibleState)
	}
}
