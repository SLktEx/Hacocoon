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
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation(workspaceID, "one", core.WorkspaceReadWrite)); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation(workspaceID, "two", core.WorkspaceReadOnly)); !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("in-flight lease must remain protective: %v", err)
	}
}

func TestWorkspaceLeaseAllowsMultipleReadOnlyEnvironments(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	workspaceID := core.WorkspaceID("workspace:demo")
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation(workspaceID, "one", core.WorkspaceReadOnly)); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation(workspaceID, "two", core.WorkspaceReadOnly)); err != nil {
		t.Fatalf("ro/ro should be allowed: %v", err)
	}
}

func TestWorkspaceLeaseRejectsDuplicateEnvironmentReservation(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation("workspace:one", "demo", core.WorkspaceReadWrite)); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginEnvironmentCreate(ctx, environmentReservation("workspace:two", "demo", core.WorkspaceReadWrite)); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("duplicate environment reservation = %v", err)
	}
}

func TestWorkspaceLeaseDeleteIsIdempotent(t *testing.T) {
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	if err := store.FinalizeEnvironmentDelete(context.Background(), "missing"); err != nil {
		t.Fatalf("delete missing lease: %v", err)
	}
}
