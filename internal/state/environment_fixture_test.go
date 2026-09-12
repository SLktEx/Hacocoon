package state

import (
	"context"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Test fixtures use the same lifecycle transitions as production. Deliberately
// malformed or historical catalogs belong in explicit serialized fixtures.
func environmentReservation(workspace core.WorkspaceID, name string, mode core.WorkspaceAccessMode) core.WorkspaceLease {
	return core.WorkspaceLease{WorkspaceID: workspace, SourcePath: "/workspace/" + string(workspace),
		EnvironmentID: name, AccessMode: mode, Owner: name, State: core.WorkspaceLeaseAcquiring,
		AcquiredAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)}
}

func commitEnvironmentFixture(t *testing.T, store *EnvironmentJSONStore, environment core.Environment) {
	t.Helper()
	ctx := context.Background()
	lease := environmentReservation(environment.Workspace.ID, environment.Name, environment.AccessMode)
	lease.SourcePath = environment.Workspace.Path
	lease.AcquiredAt = environment.CreatedAt
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = environment.RuntimeRef
	if err := store.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.State = core.WorkspaceLeaseActive
	if err := store.CommitEnvironmentCreate(ctx, environment, lease); err != nil {
		t.Fatal(err)
	}
}
