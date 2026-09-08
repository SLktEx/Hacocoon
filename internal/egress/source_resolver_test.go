package egress

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
)

type fakeRuntimeSourceResolver struct {
	ref string
	err error
}

func (f fakeRuntimeSourceResolver) ResolveRuntimeRef(context.Context, net.IP) (string, error) {
	return f.ref, f.err
}

type fakeEnvironmentLister struct {
	environments []core.Environment
	err          error
}

func (f fakeEnvironmentLister) ListEnvironments(context.Context) ([]core.Environment, error) {
	return f.environments, f.err
}

type sourceTestProvider struct{ fakeRuntimeSourceResolver }

func (p sourceTestProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{Ref: p.ref}, nil
}
func (sourceTestProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (sourceTestProvider) ShellEnvironment(context.Context, string) error  { return nil }
func (sourceTestProvider) DeleteEnvironment(context.Context, string) error { return nil }

func TestPersistedSourceResolverBindsCreatedRoutedReference(t *testing.T) {
	for _, providerID := range []string{environmentapp.ProviderIncus, "runtime.other"} {
		t.Run(providerID, func(t *testing.T) {
			provider := sourceTestProvider{fakeRuntimeSourceResolver{ref: "haco-demo"}}
			router, err := environmentapp.NewRouter(providerID, environmentapp.Register(providerID, provider))
			if err != nil {
				t.Fatal(err)
			}
			// Use the same Base router as production creation, rather than a
			// hand-written provider-local ref in persisted Environment state.
			created, err := environmentapp.NewBaseRouter(router).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus, provider,
				fakeEnvironmentLister{environments: []core.Environment{{Name: "demo", RuntimeRef: created.Ref}}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
			if providerID == environmentapp.ProviderIncus {
				if err != nil || got != "demo" {
					t.Fatalf("created Environment source = %q, %v; want demo", got, err)
				}
			} else if !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatalf("another provider's identical native ref must be denied: %q, %v", got, err)
			}
		})
	}
}

func TestPersistedSourceResolverRequiresPersistedRuntimeBinding(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{{Name: "demo", RuntimeRef: "haco-demo"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "demo" {
		t.Fatalf("environment = %q, want demo", got)
	}
}

func TestPersistedSourceResolverRejectsOrphanRuntime(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{ref: "haco-orphan"},
		fakeEnvironmentLister{environments: []core.Environment{{Name: "demo", RuntimeRef: "haco-demo"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestPersistedSourceResolverRejectsAmbiguousRuntimeBinding(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{
			{Name: "demo", RuntimeRef: "haco-demo"},
			{Name: "duplicate", RuntimeRef: "haco-demo"},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestPersistedSourceResolverFailsClosedOnProviderLookupFailure(t *testing.T) {
	lookupErr := errors.New("runtime lookup failed")
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{err: lookupErr},
		fakeEnvironmentLister{},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, lookupErr) {
		t.Fatalf("error = %v, want wrapped provider error", err)
	}
}

func TestPersistedSourceResolverFailsClosedOnStateLookupFailure(t *testing.T) {
	stateErr := errors.New("state unavailable")
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{err: stateErr},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, stateErr) {
		t.Fatalf("error = %v, want wrapped state error", err)
	}
}

func TestPersistedSourceResolverSupportsLegacyLogicalRuntimeRef(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus,
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{{Name: "demo", RuntimeRef: "demo"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "demo" {
		t.Fatalf("environment = %q, want demo", got)
	}
}

type instanceEnvironmentLister struct {
	fakeEnvironmentLister
	instance    string
	snapshot    core.Environment
	errInstance error
}

func (f *instanceEnvironmentLister) EnvironmentInstance(_ context.Context, expected core.Environment) (string, error) {
	f.snapshot = expected
	return f.instance, f.errInstance
}
func TestSourceInstanceRequiresExactPersistedCreationEvidence(t *testing.T) {
	const id = "env-11111111111111111111111111111111"
	for _, scenario := range []string{"valid", "stale", "invalid", "unsupported", "ambiguous", "orphan"} {
		t.Run(scenario, func(t *testing.T) {
			env := core.Environment{Name: "demo", RuntimeRef: "demo"}
			store := &instanceEnvironmentLister{fakeEnvironmentLister: fakeEnvironmentLister{environments: []core.Environment{env}}, instance: id}
			switch scenario {
			case "stale":
				store.errInstance = core.ErrCapabilityStale
			case "invalid":
				store.instance = ""
			case "ambiguous":
				store.environments = append(store.environments, env)
			case "orphan":
				store.environments = nil
			}
			var catalog EnvironmentLister = store
			if scenario == "unsupported" {
				catalog = store.fakeEnvironmentLister
			}
			resolver, err := NewPersistedSourceResolver(environmentapp.ProviderIncus, fakeRuntimeSourceResolver{ref: "haco-demo"}, catalog)
			if err != nil {
				t.Fatal(err)
			}
			name, instance, err := resolver.ResolveEnvironmentInstance(context.Background(), net.ParseIP("10.200.0.23"))
			if scenario == "valid" {
				if err != nil || name != "demo" || instance != id || store.snapshot.RuntimeRef != env.RuntimeRef {
					t.Fatal(name, instance, err)
				}
			} else if err == nil || name != "" || instance != "" {
				t.Fatal("unproven source accepted", name, instance, err)
			}
		})
	}
}
