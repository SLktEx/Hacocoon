package egress

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
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

func TestPersistedSourceResolverRequiresPersistedRuntimeBinding(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(
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

func TestPersistedSourceResolverMatchesRoutedIncusRuntimeBinding(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{{
			Name:       "demo",
			RuntimeRef: "haco-runtime-v1:runtime.incus:aGFjby1kZW1v",
		}}},
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

func TestPersistedSourceResolverRejectsDifferentRoutedProvider(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{{
			Name:       "demo",
			RuntimeRef: "haco-runtime-v1:runtime.other:aGFjby1kZW1v",
		}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestPersistedSourceResolverRejectsMalformedRoutedRuntimeBinding(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(
		fakeRuntimeSourceResolver{ref: "haco-demo"},
		fakeEnvironmentLister{environments: []core.Environment{{
			Name:       "demo",
			RuntimeRef: "haco-runtime-v1:runtime.incus:not-base64!",
		}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestPersistedSourceResolverRejectsOrphanRuntime(t *testing.T) {
	resolver, err := NewPersistedSourceResolver(
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
	resolver, err := NewPersistedSourceResolver(
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
	resolver, err := NewPersistedSourceResolver(
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
	resolver, err := NewPersistedSourceResolver(
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
	resolver, err := NewPersistedSourceResolver(
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
