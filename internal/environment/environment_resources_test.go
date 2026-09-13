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
	instance string
	areas    []core.EnvironmentRuntimeAttachment
}

func (*dataProvider) SupportsEnvironmentResources() bool { return true }
func (p *dataProvider) StartEnvironmentWithResources(_ context.Context, ref, instance string, areas []core.EnvironmentRuntimeAttachment) error {
	p.ref, p.instance, p.areas = ref, instance, areas
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
	err = base.StartEnvironmentWithResources(context.Background(), encodeRouteRef(testProvider, "native-ref"), "exact-instance", areas)
	if !errors.Is(err, core.ErrCapabilityStale) || provider.ref != "native-ref" || provider.instance != "exact-instance" || !reflect.DeepEqual(provider.areas, areas) {
		t.Fatal("binding or refusal lost", err)
	}
	if !errors.Is(base.StartEnvironmentWithResources(context.Background(), encodeRouteRef(ProviderIncus, "other"), "exact-instance", areas), core.ErrUnsupported) {
		t.Fatal("unsupported provider accepted")
	}
}
