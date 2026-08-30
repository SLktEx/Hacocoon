package egress

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	routedRuntimeRefPrefix = "haco-runtime-v1:"
	legacyIncusProviderID  = "runtime.incus"
)

// RuntimeSourceResolver resolves trusted provider/runtime evidence for a
// connection source. It deliberately returns the provider-local runtime ref,
// not an Environment identity supplied by the caller.
type RuntimeSourceResolver interface {
	RuntimeProviderID() string
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
	runtime      RuntimeSourceResolver
	environments EnvironmentLister
}

func NewPersistedSourceResolver(runtime RuntimeSourceResolver, environments EnvironmentLister) (*PersistedSourceResolver, error) {
	if runtime == nil || environments == nil {
		return nil, core.ErrInvalidArgument
	}
	return &PersistedSourceResolver{runtime: runtime, environments: environments}, nil
}

func (r *PersistedSourceResolver) ResolveEnvironment(ctx context.Context, source net.IP) (string, error) {
	if r == nil || r.runtime == nil || r.environments == nil || source == nil {
		return "", core.ErrPolicyDenied
	}

	providerID := strings.TrimSpace(r.runtime.RuntimeProviderID())
	if providerID == "" || providerID != r.runtime.RuntimeProviderID() || strings.ContainsAny(providerID, "\r\n\x00:") {
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
		if !matchesPersistedRuntimeRef(environment, providerID, runtimeRef) {
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

func matchesPersistedRuntimeRef(environment core.Environment, providerID, runtimeRef string) bool {
	if provider, ref, ok := decodeRoutedRuntimeRef(environment.RuntimeRef); ok {
		return provider == providerID && ref == runtimeRef
	}

	// Pre-v0.7 Environment state is Incus-backed and could persist either the
	// provider-local ref or the logical Environment name. Keep only those two
	// narrow legacy shapes; other providers must use the routed ref format so a
	// coincidentally-equal raw ref cannot cross a provider authority boundary.
	if providerID != legacyIncusProviderID {
		return false
	}
	if environment.RuntimeRef == runtimeRef {
		return true
	}
	return environment.RuntimeRef == environment.Name && runtimeRef == "haco-"+environment.Name
}

func decodeRoutedRuntimeRef(raw string) (string, string, bool) {
	if !strings.HasPrefix(raw, routedRuntimeRefPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(raw, routedRuntimeRefPrefix)
	cut := strings.IndexByte(rest, ':')
	if cut <= 0 || cut == len(rest)-1 {
		return "", "", false
	}
	provider := rest[:cut]
	if strings.TrimSpace(provider) != provider || strings.ContainsAny(provider, "\r\n\x00:") {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(rest[cut+1:])
	if err != nil || len(decoded) == 0 {
		return "", "", false
	}
	ref := string(decoded)
	if strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, "\r\n\x00") {
		return "", "", false
	}
	return provider, ref, true
}
