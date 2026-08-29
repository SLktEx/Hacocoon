package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

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
	ec2 := &fakeProvider{ref: "ec2v1.payload"}
	router, err := NewRouter(ProviderEC2, Register(ProviderIncus, incus), Register(ProviderEC2, ec2))
	if err != nil {
		t.Fatal(err)
	}
	created, err := router.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/work"})
	if err != nil {
		t.Fatal(err)
	}
	if ec2.created != 1 || incus.created != 0 || created.Ref == "ec2v1.payload" {
		t.Fatalf("created=%#v incus=%d ec2=%d", created, incus.created, ec2.created)
	}
	result, err := router.ExecEnvironment(context.Background(), created.Ref, core.ExecutionRequest{Argv: []string{"true"}})
	if err != nil || result.ExitCode != 7 || ec2.execRef != "ec2v1.payload" {
		t.Fatalf("result=%#v ref=%q err=%v", result, ec2.execRef, err)
	}
	if err := router.DeleteEnvironment(context.Background(), created.Ref); err != nil || ec2.deleted != "ec2v1.payload" {
		t.Fatalf("deleted=%q err=%v", ec2.deleted, err)
	}
}

func TestRouterTreatsPreV07BareRefsAsIncus(t *testing.T) {
	incus := &fakeProvider{}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, incus), Register(ProviderEC2, &fakeProvider{}))
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
	disabled := DisabledProvider{ID: ProviderEC2, Reason: "experimental EC2 is disabled; set HACO_EXPERIMENTAL_EC2=1 to opt in"}
	router, err := NewRouter(ProviderEC2, Register(ProviderIncus, &fakeProvider{}), Register(ProviderEC2, disabled))
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
	if _, err := router.ExecEnvironment(context.Background(), "haco-runtime-v1:runtime.ec2:not-base64%%%", core.ExecutionRequest{Argv: []string{"true"}}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}
