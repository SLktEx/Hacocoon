package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

func (f *fakeEnvironmentStore) GetWorkspaceLease(_ context.Context, environmentID string) (core.WorkspaceLease, error) {
	lease, ok := f.leases[environmentID]
	if !ok {
		return core.WorkspaceLease{}, core.ErrNotFound
	}
	return lease, nil
}

func (f *fakeEnvironmentStore) PutWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	if _, ok := f.leases[lease.EnvironmentID]; !ok {
		return core.ErrNotFound
	}
	f.leases[lease.EnvironmentID] = lease
	return nil
}

func TestCreatePersistsRuntimeReferenceInLease(t *testing.T) {
	root := t.TempDir()
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()

	if _, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root}); err != nil {
		t.Fatal(err)
	}
	lease := store.leases["demo"]
	if lease.RuntimeRef != "haco-demo" || lease.State != core.WorkspaceLeaseActive {
		t.Fatalf("lease = %#v", lease)
	}
}

func TestCreateKeepsLeaseWhenRuntimeCleanupFails(t *testing.T) {
	root := t.TempDir()
	persistErr := errors.New("disk full")
	cleanupErr := errors.New("incus delete failed")
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}, deleteErr: cleanupErr}
	store := newFakeEnvironmentStore()
	store.putErr = persistErr

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, persistErr) || !errors.Is(err, cleanupErr) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	lease, ok := store.leases["demo"]
	if !ok {
		t.Fatal("lease was released even though runtime cleanup failed")
	}
	if lease.RuntimeRef != "haco-demo" || lease.State != core.WorkspaceLeaseCleanupRequired {
		t.Fatalf("recovery lease = %#v", lease)
	}
}

type timeoutCleanupRuntime struct {
	createResult core.EnvironmentRuntime
}

func (r *timeoutCleanupRuntime) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return r.createResult, nil
}
func (*timeoutCleanupRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*timeoutCleanupRuntime) ShellEnvironment(context.Context, string) error { return nil }
func (*timeoutCleanupRuntime) DeleteEnvironment(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestCreateCleanupHasOwnDeadline(t *testing.T) {
	root := t.TempDir()
	store := newFakeEnvironmentStore()
	store.putErr = errors.New("disk full")
	service := New(&timeoutCleanupRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}, store)
	service.cleanupTimeout = 20 * time.Millisecond

	started := time.Now()
	_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup exceeded bounded deadline: %v", elapsed)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("lease was released after timed-out runtime cleanup")
	}
}

func TestCreateKeepsLeaseWhenRuntimeReportsRecoveryRequired(t *testing.T) {
	root := t.TempDir()
	runtime := &fakeEnvironmentRuntime{createErr: errors.Join(errors.New("start failed"), core.ErrRecoveryRequired)}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("lease released although runtime cleanup was uncertain")
	}
}

func TestDeleteTreatsMissingRuntimeAsAlreadyDeleted(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{deleteErr: core.ErrNotFound}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	store.leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", RuntimeRef: "haco-demo", AccessMode: core.WorkspaceReadWrite}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.environments["demo"]; ok {
		t.Fatal("metadata remains after missing runtime was treated as deleted")
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("lease remains after idempotent delete")
	}
}

func TestDeleteRecoversLeaseAfterMetadataWasAlreadyRemoved(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{deleteErr: core.ErrNotFound}
	store := newFakeEnvironmentStore()
	store.leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", RuntimeRef: "haco-demo", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseActive}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("lease remains after recovery delete")
	}
}

func TestDeleteRefusesLeaseRecoveryWithoutRuntimeProof(t *testing.T) {
	store := newFakeEnvironmentStore()
	store.leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring}

	err := New(&fakeEnvironmentRuntime{}, store).Delete(context.Background(), "demo")
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("uncertain lease was reclaimed")
	}
}

func TestCreateOnDifferentWorkspaceDoesNotReclaimInFlightLease(t *testing.T) {
	root := t.TempDir()
	onePath := filepath.Join(root, "one")
	twoPath := filepath.Join(root, "two")
	if err := os.Mkdir(onePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(twoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runtime := &blockingEnvironmentRuntime{started: make(chan struct{}), release: make(chan struct{})}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	service := New(runtime, store)

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "one", WorkspacePath: onePath})
		firstDone <- err
	}()
	<-runtime.started

	if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "two", WorkspacePath: twoPath}); err != nil {
		t.Fatalf("second workspace create: %v", err)
	}
	if _, err := store.GetWorkspaceLease(context.Background(), "one"); err != nil {
		t.Fatalf("in-flight lease was reclaimed: %v", err)
	}

	close(runtime.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first create: %v", err)
	}
}
