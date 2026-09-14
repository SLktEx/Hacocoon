package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"testing"
)

type dataProvider struct {
	fakeProvider
	binding core.EnvironmentResourceBinding
}

func (*dataProvider) SupportsEnvironmentResources() bool { return true }
func (p *dataProvider) StartEnvironmentWithResources(_ context.Context, ref string, binding core.EnvironmentResourceBinding) error {
	p.ref, p.binding = ref, binding
	return core.ErrCapabilityStale
}

func TestEnvironmentResourcesRouteToExactProvider(t *testing.T) {
	provider := &dataProvider{}
	r, err := NewRouter(testProvider, Register(testProvider, provider), Register(ProviderIncus, &fakeProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	base := NewBaseRouter(r)
	if !base.SupportsEnvironmentResources() {
		t.Fatal("support lost")
	}
	areas := []core.EnvironmentRuntimeAttachment{{Attachment: core.EnvironmentAttachment{Key: "area"}}}
	binding := core.EnvironmentResourceBinding{InstanceID: "exact-instance", WorkspacePath: "managed:work", ReadOnly: true, Attachments: areas}
	err = base.StartEnvironmentWithResources(context.Background(), encodeRouteRef(testProvider, "native-ref"), binding)
	if !errors.Is(err, core.ErrCapabilityStale) || provider.ref != "native-ref" || !reflect.DeepEqual(provider.binding, binding) {
		t.Fatal("binding or refusal lost", err)
	}
	if !errors.Is(base.StartEnvironmentWithResources(context.Background(), encodeRouteRef(ProviderIncus, "other"), binding), core.ErrUnsupported) {
		t.Fatal("unsupported provider accepted")
	}
}
