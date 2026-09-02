package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCreateRetainsKnownRuntimeRefWhenProviderRequiresRecovery(t *testing.T) {
	providerErr := errors.New("provider reconciliation became ambiguous")
	runtime := &fakeEnvironmentRuntime{
		createResult: core.EnvironmentRuntime{Ref: "haco-demo"},
		createErr:    errors.Join(providerErr, core.ErrRecoveryRequired),
	}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if !errors.Is(err, providerErr) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	lease, ok := store.leases["demo"]
	if !ok {
		t.Fatal("Workspace reservation was released although provider ownership is ambiguous")
	}
	if lease.State != core.WorkspaceLeaseCleanupRequired || lease.RuntimeRef != "haco-demo" {
		t.Fatalf("recovery lease = %#v, want cleanup-required with exact runtime ref", lease)
	}
	if len(runtime.deleteRefs) != 0 {
		t.Fatalf("Core retried cleanup even though provider reported recovery-required: %#v", runtime.deleteRefs)
	}
}

func TestCreateCleansKnownRuntimeWhenProviderReturnsRefWithOrdinaryError(t *testing.T) {
	providerErr := errors.New("provider failed after creating runtime")
	runtime := &fakeEnvironmentRuntime{
		createResult: core.EnvironmentRuntime{Ref: "haco-demo"},
		createErr:    providerErr,
	}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %v", err)
	}
	if len(runtime.deleteRefs) != 1 || runtime.deleteRefs[0] != "haco-demo" {
		t.Fatalf("cleanup refs = %#v, want exact provider runtime", runtime.deleteRefs)
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("Workspace lease remained after confirmed runtime cleanup")
	}
}

func TestCreateRetainsKnownRuntimeWhenCoreCleanupAlsoFails(t *testing.T) {
	providerErr := errors.New("provider failed after creating runtime")
	cleanupErr := errors.New("runtime delete failed")
	runtime := &fakeEnvironmentRuntime{
		createResult: core.EnvironmentRuntime{Ref: "haco-demo"},
		createErr:    providerErr,
		deleteErr:    cleanupErr,
	}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if !errors.Is(err, providerErr) || !errors.Is(err, cleanupErr) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	lease, ok := store.leases["demo"]
	if !ok {
		t.Fatal("Workspace reservation was released after uncertain runtime cleanup")
	}
	if lease.State != core.WorkspaceLeaseCleanupRequired || lease.RuntimeRef != "haco-demo" {
		t.Fatalf("recovery lease = %#v, want cleanup-required with exact runtime ref", lease)
	}
}
