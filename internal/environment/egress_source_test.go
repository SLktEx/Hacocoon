package environment

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type egressSourceTestStore struct{ environments []core.Environment }

func (s egressSourceTestStore) ListEnvironments(context.Context) ([]core.Environment, error) {
	return append([]core.Environment(nil), s.environments...), nil
}

type egressSourceTestProvider struct {
	ref string
	err error
}

func (p *egressSourceTestProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, core.ErrUnsupported
}
func (p *egressSourceTestProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, core.ErrUnsupported
}
func (p *egressSourceTestProvider) ShellEnvironment(context.Context, string) error { return core.ErrUnsupported }
func (p *egressSourceTestProvider) DeleteEnvironment(context.Context, string) error { return core.ErrUnsupported }
func (p *egressSourceTestProvider) ResolveSourceIP(context.Context, net.IP) (string, error) {
	return p.ref, p.err
}

func TestEgressSourceResolverRequiresPersistedRuntimeMatch(t *testing.T) {
	provider := &egressSourceTestProvider{ref: "haco-env-a"}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewEgressSourceResolver(router, egressSourceTestStore{environments: []core.Environment{
		{Name: "env-a", RuntimeRef: encodeRouteRef(ProviderIncus, "haco-env-a")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil || got != "env-a" {
		t.Fatalf("environment=%q err=%v", got, err)
	}
}

func TestEgressSourceResolverRejectsUnpersistedRuntime(t *testing.T) {
	provider := &egressSourceTestProvider{ref: "haco-orphan"}
	router, _ := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	resolver, _ := NewEgressSourceResolver(router, egressSourceTestStore{environments: []core.Environment{
		{Name: "env-a", RuntimeRef: encodeRouteRef(ProviderIncus, "haco-env-a")},
	}})
	_, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("err=%v want ErrPolicyDenied", err)
	}
}

func TestEgressSourceResolverFailsClosedOnAmbiguousPersistedBinding(t *testing.T) {
	provider := &egressSourceTestProvider{ref: "haco-shared"}
	router, _ := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	resolver, _ := NewEgressSourceResolver(router, egressSourceTestStore{environments: []core.Environment{
		{Name: "env-a", RuntimeRef: encodeRouteRef(ProviderIncus, "haco-shared")},
		{Name: "env-b", RuntimeRef: encodeRouteRef(ProviderIncus, "haco-shared")},
	}})
	_, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestEgressSourceResolverSupportsPreV07IncusRuntimeRef(t *testing.T) {
	provider := &egressSourceTestProvider{ref: "haco-legacy"}
	router, _ := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	resolver, _ := NewEgressSourceResolver(router, egressSourceTestStore{environments: []core.Environment{
		{Name: "legacy", RuntimeRef: "haco-legacy"},
	}})
	got, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil || got != "legacy" {
		t.Fatalf("environment=%q err=%v", got, err)
	}
}

func TestEgressSourceResolverPropagatesProviderFailure(t *testing.T) {
	provider := &egressSourceTestProvider{err: errors.New("incus unavailable")}
	router, _ := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	resolver, _ := NewEgressSourceResolver(router, egressSourceTestStore{environments: []core.Environment{
		{Name: "env-a", RuntimeRef: encodeRouteRef(ProviderIncus, "haco-env-a")},
	}})
	if _, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23")); err == nil {
		t.Fatal("expected provider failure")
	}
}
