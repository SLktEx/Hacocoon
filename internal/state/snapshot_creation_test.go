package state

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"reflect"
	"testing"
	"time"
)

func savedCreationFixture(t *testing.T) (*EnvironmentJSONStore, core.Snapshot, core.WorkspaceLease) {
	t.Helper()
	ctx := context.Background()
	st, saved := snapshotCatalogFixture(t)
	mustSnapshot(t, st.BeginSnapshot(ctx, saved))
	for _, c := range saved.Components {
		mustSnapshot(t, st.RecordSnapshotComponent(ctx, saved.ID, c, "created"))
		c.State = "created"
		mustSnapshot(t, st.RecordSnapshotComponent(ctx, saved.ID, c, "verified"))
	}
	mustSnapshot(t, st.CommitSnapshot(ctx, saved.ID))
	var err error
	saved, err = st.GetSnapshot(ctx, saved.ID)
	mustSnapshot(t, err)
	mustSnapshot(t, st.FinalizeEnvironmentDelete(ctx, "dev"))
	generation, err := core.NewEnvironmentInstanceID()
	mustSnapshot(t, err)
	lease := core.WorkspaceLease{SnapshotSource: saved.ID, InstanceID: generation, EnvironmentID: "dev", WorkspaceID: "new-work", SourcePath: "managed:new-work", Owner: "dev", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	return st, saved, lease
}
func TestSnapshotCreationLeaseProtectsSourceAndReusesName(t *testing.T) {
	for _, finish := range []string{"publish", "cleanup"} {
		t.Run(finish, func(t *testing.T) {
			ctx := context.Background()
			st, saved, lease := savedCreationFixture(t)
			if err := st.BeginEnvironmentCreate(ctx, lease); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal("ordinary reservation accepted source", err)
			}
			stale := saved
			stale.Source.InstanceID = lease.InstanceID
			if err := st.BeginEnvironmentCreateFromSnapshot(ctx, lease, stale); err == nil {
				t.Fatal("old generation accepted")
			}
			mustSnapshot(t, st.BeginEnvironmentCreateFromSnapshot(ctx, lease, saved))
			st = NewEnvironmentJSONStore(st.path)
			if err := st.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal("source deletion during creation", err)
			}
			lease.RuntimeRef = "haco-dev"
			changed := lease
			changed.SnapshotSource = ""
			if err := st.RecordEnvironmentRuntime(ctx, changed); !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal("reservation changed", err)
			}
			mustSnapshot(t, st.RecordEnvironmentRuntime(ctx, lease))
			if finish == "publish" {
				lease.State = core.WorkspaceLeaseActive
				env := core.Environment{Name: lease.EnvironmentID, RuntimeRef: lease.RuntimeRef, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, CreatedAt: lease.AcquiredAt}
				mustSnapshot(t, st.CommitEnvironmentCreate(ctx, env, lease))
				mustSnapshot(t, st.CommitEnvironmentCreate(ctx, env, lease))
				current, err := st.GetWorkspaceLease(ctx, lease.EnvironmentID)
				mustSnapshot(t, err)
				if current.SnapshotSource != "" || current.InstanceID != lease.InstanceID || current.InstanceID == saved.Source.InstanceID {
					t.Fatal("publication identity", current)
				}
			} else {
				lease.State = core.WorkspaceLeaseCleanupRequired
				mustSnapshot(t, st.MarkEnvironmentRecoveryRequired(ctx, lease))
				if err := st.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
					t.Fatal("ambiguous cleanup released source", err)
				}
				mustSnapshot(t, st.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID))
			}
			mustSnapshot(t, st.BeginSnapshotDelete(ctx, saved.ID))
		})
	}
}
func TestSnapshotCreationSchemaElevenPreservationAndDowngradeRefusal(t *testing.T) {
	ctx := context.Background()
	st, saved, lease := savedCreationFixture(t)
	raw, err := os.ReadFile(st.path)
	mustSnapshot(t, err)
	var old environmentFileState
	mustSnapshot(t, json.Unmarshal(raw, &old))
	old.Version = 11
	raw, err = json.Marshal(old)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(st.path, raw, 0600))
	mustSnapshot(t, st.BeginEnvironmentCreateFromSnapshot(ctx, lease, saved))
	raw, err = os.ReadFile(st.path)
	mustSnapshot(t, err)
	var current environmentFileState
	mustSnapshot(t, json.Unmarshal(raw, &current))
	if current.Version != environmentStateVersion || !reflect.DeepEqual(old.Snapshots, current.Snapshots) || !reflect.DeepEqual(old.Environments, current.Environments) {
		t.Fatal("existing saved data changed")
	}
	current.Version = 12
	raw, err = json.Marshal(current)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(st.path, raw, 0600))
	preserved, err := st.GetWorkspaceLease(ctx, lease.EnvironmentID)
	mustSnapshot(t, err)
	if preserved.SnapshotSource != saved.ID || preserved.InstanceID != lease.InstanceID {
		t.Fatal("schema 12 source identity lost")
	}
	current.Version = 11
	raw, err = json.Marshal(current)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(st.path, raw, 0600))
	if _, err := st.GetWorkspaceLease(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("new lease accepted as old schema", err)
	}
}

func TestSnapshotCreationAndDeleteAreAtomic(t *testing.T) {
	for attempt := 0; attempt < 8; attempt++ {
		st, saved, lease := savedCreationFixture(t)
		ctx := context.Background()
		start := make(chan struct{})
		created := make(chan error, 1)
		deleted := make(chan error, 1)
		go func() { <-start; created <- st.BeginEnvironmentCreateFromSnapshot(ctx, lease, saved) }()
		go func() { <-start; deleted <- NewEnvironmentJSONStore(st.path).BeginSnapshotDelete(ctx, saved.ID) }()
		close(start)
		createErr, deleteErr := <-created, <-deleted
		if createErr == nil {
			if !errors.Is(deleteErr, core.ErrStorageBusy) {
				t.Fatal("delete won after source reservation", deleteErr)
			}
		} else if deleteErr != nil || !errors.Is(createErr, core.ErrCapabilityStale) {
			t.Fatal("unexpected race outcome", createErr, deleteErr)
		}
	}
}
