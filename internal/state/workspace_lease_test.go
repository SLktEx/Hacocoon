package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestWorkspaceLeaseIsNotReclaimedOnlyBecauseEnvironmentMetadataIsMissing(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	workspaceID := core.WorkspaceID("workspace:demo")
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{
		WorkspaceID:   workspaceID,
		EnvironmentID: "one",
		AccessMode:    core.WorkspaceReadWrite,
		State:         core.WorkspaceLeaseAcquiring,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{
		WorkspaceID:   workspaceID,
		EnvironmentID: "two",
		AccessMode:    core.WorkspaceReadOnly,
	}); !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("in-flight lease must remain protective: %v", err)
	}
}

func TestWorkspaceLeaseAllowsMultipleReadOnlyEnvironments(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	workspaceID := core.WorkspaceID("workspace:demo")
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "one", AccessMode: core.WorkspaceReadOnly}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: workspaceID, EnvironmentID: "two", AccessMode: core.WorkspaceReadOnly}); err != nil {
		t.Fatalf("ro/ro should be allowed: %v", err)
	}
}

func TestWorkspaceLeaseRejectsDuplicateEnvironmentReservation(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: "workspace:one", EnvironmentID: "demo", AccessMode: core.WorkspaceReadWrite}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireWorkspaceLease(ctx, core.WorkspaceLease{WorkspaceID: "workspace:two", EnvironmentID: "demo", AccessMode: core.WorkspaceReadWrite}); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("duplicate environment reservation = %v", err)
	}
}

func TestWorkspaceLeaseDeleteIsIdempotent(t *testing.T) {
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	if err := store.DeleteWorkspaceLease(context.Background(), "missing"); err != nil {
		t.Fatalf("delete missing lease: %v", err)
	}
}
