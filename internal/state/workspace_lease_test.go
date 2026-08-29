package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestWorkspaceLeaseConflictsAndRecovery(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	workspaceID := core.WorkspaceID("path:demo")
	active := core.Environment{Name: "one", Workspace: core.Workspace{ID: workspaceID, Path: "/workspace"}}
	if err := store.PutEnvironment(ctx, active); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "one", AccessMode: core.WorkspaceReadWrite}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "two", AccessMode: core.WorkspaceReadOnly}); !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("rw/ro conflict = %v", err)
	}
	if err := store.DeleteEnvironment(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "two", AccessMode: core.WorkspaceReadOnly}); err != nil {
		t.Fatalf("stale lease was not recovered: %v", err)
	}
}

func TestWorkspaceLeaseAllowsMultipleReadOnlyEnvironments(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	workspaceID := core.WorkspaceID("path:demo")
	if err := store.PutEnvironment(ctx, core.Environment{Name: "one", Workspace: core.Workspace{ID: workspaceID}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "one", AccessMode: core.WorkspaceReadOnly}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "two", AccessMode: core.WorkspaceReadOnly}); err != nil {
		t.Fatalf("ro/ro should be allowed: %v", err)
	}
}
