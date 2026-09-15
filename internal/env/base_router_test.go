package environment

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type baseTestProvider struct {
	created core.EnvironmentRuntime
	bases   []core.BaseInfo
	inspect core.BaseInfo
	spec    core.EnvironmentRuntimeSpec
}

func (p *baseTestProvider) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	p.spec = spec
	return p.created, nil
}
func (*baseTestProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*baseTestProvider) ShellEnvironment(context.Context, string) error  { return nil }
func (*baseTestProvider) DeleteEnvironment(context.Context, string) error { return nil }
func (p *baseTestProvider) ListBases(context.Context) ([]core.BaseInfo, error) {
	return p.bases, nil
}
func (p *baseTestProvider) InspectBase(context.Context, core.BaseName) (core.BaseInfo, error) {
	return p.inspect, nil
}

func TestBaseRouterPreservesProviderNeutralBaseMetadata(t *testing.T) {
	base := core.BaseRef{Name: "my-dev", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	provider := &baseTestProvider{created: core.EnvironmentRuntime{Ref: "native-ref", Base: &base}}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	if err != nil {
		t.Fatal(err)
	}
	created, err := NewBaseRouter(router).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/demo", Base: "my-dev"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Base == nil || *created.Base != base {
		t.Fatalf("Base=%#v want=%#v", created.Base, base)
	}
	if provider.spec.Base != "my-dev" {
		t.Fatalf("provider Base=%q", provider.spec.Base)
	}
	providerID, nativeRef, err := decodeRouteRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if providerID != ProviderIncus || nativeRef != "native-ref" {
		t.Fatalf("route=%q %q", providerID, nativeRef)
	}
}

func TestBaseRouterDelegatesCatalogToSelectedProvider(t *testing.T) {
	provider := &baseTestProvider{
		bases:   []core.BaseInfo{{Name: "my-dev"}},
		inspect: core.BaseInfo{Name: "my-dev", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, provider))
	if err != nil {
		t.Fatal(err)
	}
	baseRouter := NewBaseRouter(router)
	bases, err := baseRouter.ListBases(context.Background())
	if err != nil || len(bases) != 1 || bases[0].Name != "my-dev" {
		t.Fatalf("bases=%#v err=%v", bases, err)
	}
	info, err := baseRouter.InspectBase(context.Background(), "my-dev")
	if err != nil || info.Revision == "" {
		t.Fatalf("info=%#v err=%v", info, err)
	}
}
