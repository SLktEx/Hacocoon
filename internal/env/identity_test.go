package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type identityProvider struct {
	fakeProvider
	ref, id string
}

func (p *identityProvider) VerifyEnvironmentIdentity(_ context.Context, ref, id string) error {
	p.ref, p.id = ref, id
	return core.ErrCapabilityStale
}
func TestIdentityVerificationUsesPersistedProviderRoute(t *testing.T) {
	chosen := &identityProvider{}
	other := &identityProvider{}
	router, err := NewRouter("other", Register(testProvider, chosen), Register("other", other))
	if err != nil {
		t.Fatal(err)
	}
	id := "env-11111111111111111111111111111111"
	err = NewBaseRouter(router).VerifyEnvironmentIdentity(context.Background(), encodeRouteRef(testProvider, "owned-native"), id)
	if !errors.Is(err, core.ErrCapabilityStale) || chosen.ref != "owned-native" || chosen.id != id || other.ref != "" {
		t.Fatal("identity routed to wrong provider", err)
	}
	unsupported, err := NewRouter(testProvider, Register(testProvider, &fakeProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(unsupported.VerifyEnvironmentIdentity(context.Background(), encodeRouteRef(testProvider, "owned-native"), id), core.ErrUnsupported) {
		t.Fatal("provider without identity accepted")
	}
}
