package egress

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
)

// RuntimeSourceResolver resolves trusted provider/runtime evidence for a
// connection source. It deliberately returns the provider-local runtime ref,
// not an Environment identity supplied by the caller.
type RuntimeSourceResolver interface {
	ResolveRuntimeRef(context.Context, net.IP) (string, error)
}

// EnvironmentLister exposes persisted managed Environment state used to bind
// runtime evidence to Hacocoon authority.
type EnvironmentLister interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
}

// PersistedSourceResolver requires both runtime/provider evidence and exactly
// one matching persisted Environment before granting an Environment identity.
type PersistedSourceResolver struct {
	provider     string
	runtime      RuntimeSourceResolver
	environments EnvironmentLister
}

func NewPersistedSourceResolver(provider string, runtime RuntimeSourceResolver, environments EnvironmentLister) (*PersistedSourceResolver, error) {
	if strings.TrimSpace(provider) == "" || runtime == nil || environments == nil {
		return nil, core.ErrInvalidArgument
	}
	return &PersistedSourceResolver{provider: provider, runtime: runtime, environments: environments}, nil
}

func (r *PersistedSourceResolver) ResolveEnvironment(ctx context.Context, source net.IP) (string, error) {
	if r == nil || r.runtime == nil || r.environments == nil || source == nil {
		return "", core.ErrPolicyDenied
	}

	runtimeRef, err := r.runtime.ResolveRuntimeRef(ctx, source)
	if err != nil {
		return "", fmt.Errorf("resolve egress runtime source: %w", err)
	}
	if runtimeRef == "" {
		return "", core.ErrPolicyDenied
	}

	environments, err := r.environments.ListEnvironments(ctx)
	if err != nil {
		return "", fmt.Errorf("read persisted Environment state for egress source: %w", err)
	}

	matched := ""
	matches := 0
	for _, environment := range environments {
		if !matchesPersistedRuntimeRef(environment, r.provider, runtimeRef) {
			continue
		}
		matches++
		matched = environment.Name
	}
	if matches != 1 || matched == "" {
		return "", core.ErrPolicyDenied
	}
	return matched, nil
}

func matchesPersistedRuntimeRef(environment core.Environment, provider, runtimeRef string) bool {
	if environmentapp.MatchesRuntimeRef(environment.RuntimeRef, provider, runtimeRef) {
		return true
	}
	// Pre-v0.7 Environment state could persist the logical Environment name as
	// the Incus runtime ref. Keep that narrow compatibility shape without
	// accepting arbitrary aliases: the provider ref must be exactly haco-<name>.
	return provider == environmentapp.ProviderIncus && environment.RuntimeRef == environment.Name && runtimeRef == "haco-"+environment.Name
}
