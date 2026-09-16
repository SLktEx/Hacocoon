package environment

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type baseTestProvider struct {
	created     core.EnvironmentRuntime
	bases       []core.BaseInfo
	inspect     core.BaseInfo
	spec        core.EnvironmentRuntimeSpec
	createErr   error
	catalogErr  error
	createCalls int
}

func (p *baseTestProvider) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	p.spec = spec
	p.createCalls++
	return p.created, p.createErr
}
func (*baseTestProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*baseTestProvider) ShellEnvironment(context.Context, string) error  { return nil }
func (*baseTestProvider) DeleteEnvironment(context.Context, string) error { return nil }
func (p *baseTestProvider) ListBases(context.Context) ([]core.BaseInfo, error) {
	return p.bases, p.catalogErr
}
func (p *baseTestProvider) InspectBase(context.Context, core.BaseName) (core.BaseInfo, error) {
	return p.inspect, p.catalogErr
}

func TestRouterPreservesProviderNeutralBaseMetadata(t *testing.T) {
	base := core.BaseRef{Name: "my-dev", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	budget, err := core.ResolveResourceBudget(core.ResourceBudget{})
	if err != nil {
		t.Fatal(err)
	}
	provider := &baseTestProvider{created: core.EnvironmentRuntime{Ref: "native-ref", Base: &base, Resources: budget}}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	if err != nil {
		t.Fatal(err)
	}
	created, err := router.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/demo", Base: "my-dev", Resources: budget})
	if err != nil {
		t.Fatal(err)
	}
	if created.Base == nil || *created.Base != base {
		t.Fatalf("Base=%#v want=%#v", created.Base, base)
	}
	if provider.spec.Base != "my-dev" {
		t.Fatalf("provider Base=%q", provider.spec.Base)
	}
	if created.Resources != budget || provider.spec.Resources != budget {
		t.Fatalf("resource budget lost: returned=%+v requested=%+v want=%+v", created.Resources, provider.spec.Resources, budget)
	}
	providerID, nativeRef, err := decodeRouteRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if providerID != ProviderIncus || nativeRef != "native-ref" {
		t.Fatalf("route=%q %q", providerID, nativeRef)
	}
}

func TestRouterDelegatesCatalogToSelectedProvider(t *testing.T) {
	provider := &baseTestProvider{
		bases:   []core.BaseInfo{{Name: "my-dev"}},
		inspect: core.BaseInfo{Name: "my-dev", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	if err != nil {
		t.Fatal(err)
	}
	bases, err := router.ListBases(context.Background())
	if err != nil || len(bases) != 1 || bases[0].Name != "my-dev" {
		t.Fatalf("bases=%#v err=%v", bases, err)
	}
	info, err := router.InspectBase(context.Background(), "my-dev")
	if err != nil || info.Revision == "" {
		t.Fatalf("info=%#v err=%v", info, err)
	}
}

type publishingProvider struct {
	baseTestProvider
	env     core.Environment
	lease   core.WorkspaceLease
	name    core.BaseName
	calls   int
	failure error
}

func (p *publishingProvider) PublishBase(_ context.Context, env core.Environment, lease core.WorkspaceLease, name core.BaseName) (core.BaseInfo, error) {
	p.env, p.lease, p.name = env, lease, name
	p.calls++
	return core.BaseInfo{Name: name, Revision: "immutable-image"}, p.failure
}

func TestBasePublicationPreservesExactLeaseOwnership(t *testing.T) {
	p, other := &publishingProvider{}, &publishingProvider{}
	r, err := NewRouter("other", Register("other", other), Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	env := core.Environment{Name: "builder", RuntimeRef: encodeRouteRef(testProvider, "native-builder"), Workspace: core.Workspace{ID: "work", Path: "temporary-work"}}
	lease := core.WorkspaceLease{RuntimeRef: env.RuntimeRef, EnvironmentID: env.Name, WorkspaceID: env.Workspace.ID, SourcePath: env.Workspace.Path, InstanceID: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: core.WorkspaceLeaseActive}
	for _, failure := range []error{nil, core.ErrCapabilityStale} {
		p.calls, p.failure = 0, failure
		got, err := r.PublishBase(context.Background(), env, lease, "new-base")
		wantEnv, wantLease := env, lease
		wantEnv.RuntimeRef, wantLease.RuntimeRef = "native-builder", "native-builder"
		if !errors.Is(err, failure) || got != (core.BaseInfo{Name: "new-base", Revision: "immutable-image"}) || p.calls != 1 || other.calls != 0 || p.name != "new-base" || !reflect.DeepEqual(p.env, wantEnv) || !reflect.DeepEqual(p.lease, wantLease) {
			t.Fatal("publication lost route, ownership, result or error", got, err, p, other)
		}
	}
	for _, mismatch := range []string{"", "native-builder", encodeRouteRef("other", "native-builder"), encodeRouteRef(testProvider, "different-builder")} {
		p.calls = 0
		bad := lease
		bad.RuntimeRef = mismatch
		if _, err := r.PublishBase(context.Background(), env, bad, "new-base"); !errors.Is(err, core.ErrInvalidArgument) || p.calls != 0 || other.calls != 0 {
			t.Fatalf("mismatched lease %q reached publisher: %v (calls %d/%d)", mismatch, err, p.calls, other.calls)
		}
	}
}

func TestBaseCapabilitiesDoNotFallBackToAnotherProvider(t *testing.T) {
	p := &baseTestProvider{catalogErr: core.ErrRuntimeUnavailable}
	r, _ := NewRouter(testProvider, Register(testProvider, p))
	if _, err := r.ListBases(context.Background()); !errors.Is(err, p.catalogErr) {
		t.Fatal("catalog failure lost", err)
	}
	if _, err := r.InspectBase(context.Background(), "base"); !errors.Is(err, p.catalogErr) {
		t.Fatal("inspection failure lost", err)
	}
	limited, _ := NewRouter(testProvider, Register(testProvider, &fakeProvider{}), Register("other", p))
	for _, tc := range []struct {
		router *Router
		want   error
	}{
		{nil, core.ErrRuntimeUnavailable}, {&Router{defaultProvider: "missing"}, core.ErrUnsupported}, {limited, core.ErrUnsupported},
	} {
		if _, err := tc.router.ListBases(context.Background()); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
		if _, err := tc.router.InspectBase(context.Background(), "base"); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
		raw := encodeRouteRef(testProvider, "native-builder")
		if _, err := tc.router.PublishBase(context.Background(), core.Environment{RuntimeRef: raw}, core.WorkspaceLease{RuntimeRef: raw}, "base"); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}
