package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const testProvider = "runtime.test"

type fakeProvider struct {
	created int
	execRef string
	deleted string
	ref     string
}

func (f *fakeProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	f.created++
	ref := f.ref
	if ref == "" {
		ref = "inner-ref"
	}
	return core.EnvironmentRuntime{Ref: ref}, nil
}
func (f *fakeProvider) ExecEnvironment(_ context.Context, ref string, _ core.ExecutionRequest) (core.ExecutionResult, error) {
	f.execRef = ref
	return core.ExecutionResult{ExitCode: 7}, nil
}
func (*fakeProvider) ShellEnvironment(context.Context, string) error { return nil }
func (f *fakeProvider) DeleteEnvironment(_ context.Context, ref string) error {
	f.deleted = ref
	return nil
}
func (*fakeProvider) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	return core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}, nil
}

func TestRouterUsesConfiguredProviderAndKeepsProviderOutOfCoreState(t *testing.T) {
	incus := &fakeProvider{ref: "haco-demo"}
	alternate := &fakeProvider{ref: "provider-ref"}
	router, err := NewRouter(testProvider, Register(ProviderIncus, incus), Register(testProvider, alternate))
	if err != nil {
		t.Fatal(err)
	}
	created, err := router.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/work"})
	if err != nil {
		t.Fatal(err)
	}
	if alternate.created != 1 || incus.created != 0 || created.Ref == "provider-ref" {
		t.Fatalf("created=%#v incus=%d alternate=%d", created, incus.created, alternate.created)
	}
	result, err := router.ExecEnvironment(context.Background(), created.Ref, core.ExecutionRequest{Argv: []string{"true"}})
	if err != nil || result.ExitCode != 7 || alternate.execRef != "provider-ref" {
		t.Fatalf("result=%#v ref=%q err=%v", result, alternate.execRef, err)
	}
	if err := router.DeleteEnvironment(context.Background(), created.Ref); err != nil || alternate.deleted != "provider-ref" {
		t.Fatalf("deleted=%q err=%v", alternate.deleted, err)
	}
}

func TestRouterTreatsPreV07BareRefsAsIncus(t *testing.T) {
	incus := &fakeProvider{}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, incus), Register(testProvider, &fakeProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.ExecEnvironment(context.Background(), "haco-old", core.ExecutionRequest{Argv: []string{"true"}}); err != nil {
		t.Fatal(err)
	}
	if incus.execRef != "haco-old" {
		t.Fatalf("ref=%q", incus.execRef)
	}
}

func TestDisabledProviderFailsClosed(t *testing.T) {
	disabled := DisabledProvider{ID: testProvider, Reason: "test provider is disabled"}
	router, err := NewRouter(testProvider, Register(ProviderIncus, &fakeProvider{}), Register(testProvider, disabled))
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/work"})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("err=%v", err)
	}
}

func TestRouterRejectsUnknownDefaultAndMalformedWrappedRef(t *testing.T) {
	if _, err := NewRouter("runtime.unknown", Register(ProviderIncus, &fakeProvider{})); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	router, _ := NewRouter(ProviderIncus, Register(ProviderIncus, &fakeProvider{}))
	if _, err := router.ExecEnvironment(context.Background(), "haco-runtime-v1:runtime.test:not-base64%%%", core.ExecutionRequest{Argv: []string{"true"}}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}
