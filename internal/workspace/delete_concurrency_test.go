package workspace

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type blockingDeleteRuntime struct {
	calls   chan string
	release chan struct{}
}

func (*blockingDeleteRuntime) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, nil
}
func (*blockingDeleteRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*blockingDeleteRuntime) ShellEnvironment(context.Context, string) error { return nil }
func (r *blockingDeleteRuntime) DeleteEnvironment(_ context.Context, ref string) error {
	r.calls <- ref
	<-r.release
	return nil
}

func TestDeleteSerializesConcurrentRuntimeDeletion(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	now := time.Now().UTC()
	workspaceID := core.WorkspaceID("delete-concurrency-test")
	environment := core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{ID: workspaceID, Path: t.TempDir()},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "runtime-demo",
		CreatedAt:  now,
	}
	lease := core.WorkspaceLease{
		WorkspaceID:   workspaceID,
		SourcePath:    environment.Workspace.Path,
		EnvironmentID: environment.Name,
		AccessMode:    core.WorkspaceReadWrite,
		Owner:         environment.Name,
		RuntimeRef:    environment.RuntimeRef,
		State:         core.WorkspaceLeaseActive,
		AcquiredAt:    now,
	}
	if err := store.AcquireWorkspaceLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if err := store.PutEnvironment(ctx, environment); err != nil {
		t.Fatal(err)
	}

	runtime := &blockingDeleteRuntime{calls: make(chan string, 2), release: make(chan struct{})}
	service := New(runtime, store)
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { firstDone <- service.Delete(ctx, "demo") }()
	if ref := <-runtime.calls; ref != environment.RuntimeRef {
		t.Fatalf("first runtime delete ref = %q", ref)
	}
	go func() { secondDone <- service.Delete(ctx, "demo") }()

	secondEnteredRuntime := false
	select {
	case <-runtime.calls:
		secondEnteredRuntime = true
	case <-time.After(75 * time.Millisecond):
	}
	close(runtime.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if secondEnteredRuntime {
		t.Fatal("concurrent delete reached runtime twice before the first delete completed")
	}
	select {
	case ref := <-runtime.calls:
		t.Fatalf("runtime delete was executed twice; second ref=%q", ref)
	default:
	}
}
