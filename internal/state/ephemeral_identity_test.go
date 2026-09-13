package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func runIdentityFixture(t *testing.T) (*EnvironmentJSONStore, core.EphemeralRun, core.WorkspaceLease) {
	t.Helper()
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	run := core.EphemeralRun{EnvironmentID: "run-test", InstanceID: id, State: core.EphemeralRunCreating, CreatedAt: time.Now().UTC()}
	lease := core.WorkspaceLease{EnvironmentID: run.EnvironmentID, InstanceID: id, Ephemeral: true, WorkspaceID: "ws-demo", SourcePath: "/workspace/demo", AccessMode: core.WorkspaceReadWrite, Owner: "run-owner", AcquiredAt: run.CreatedAt, State: core.WorkspaceLeaseAcquiring}
	return s, run, lease
}

func TestRunIdentityCannotBeReassignedDuringLifecycle(t *testing.T) {
	s, run, lease := runIdentityFixture(t)
	ctx := context.Background()
	if err := s.PutEphemeralRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*core.EphemeralRun){
		func(r *core.EphemeralRun) { r.InstanceID, _ = core.NewEnvironmentInstanceID() },
		func(r *core.EphemeralRun) { r.InstanceID = "" },
		func(r *core.EphemeralRun) { r.CreatedAt = r.CreatedAt.Add(time.Second) },
		func(r *core.EphemeralRun) { w, _ := core.NewTemporaryWorkspace(); r.TemporaryWorkspace = &w },
	} {
		changed := run
		change(&changed)
		if err := s.PutEphemeralRun(ctx, changed); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatal("run identity replaced", err)
		}
	}
	if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	changed := lease
	changed.Ephemeral = false
	changed.RuntimeRef = "owned-provider"
	if err := s.RecordEnvironmentRuntime(ctx, changed); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("run owner class changed", err)
	}
	if err := s.DeleteEphemeralRun(ctx, run.EnvironmentID); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("live run marker deleted", err)
	}
	lease.RuntimeRef = "owned-provider"
	if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
}

func TestRunMarkerCannotAdoptAnOrdinaryReservation(t *testing.T) {
	s, run, lease := runIdentityFixture(t)
	lease.Ephemeral = false
	if err := s.BeginEnvironmentCreate(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	if err := s.PutEphemeralRun(context.Background(), run); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("run adopted ordinary creation", err)
	}
}

func TestRunCatalogRejectsMissingOrChangedOwnershipAfterRestart(t *testing.T) {
	for _, mode := range []string{"missing-marker", "changed-generation", "ordinary-lease", "changed-workspace"} {
		t.Run(mode, func(t *testing.T) {
			s, run, lease := runIdentityFixture(t)
			if mode == "changed-workspace" {
				w, _ := core.NewTemporaryWorkspace()
				run.TemporaryWorkspace = &w
				lease.WorkspaceID = w.ID
				lease.SourcePath = w.Path
			}
			ctx := context.Background()
			if err := s.PutEphemeralRun(ctx, run); err != nil {
				t.Fatal(err)
			}
			if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(s.path)
			if err != nil {
				t.Fatal(err)
			}
			var data environmentFileState
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "missing-marker":
				delete(data.EphemeralRuns, run.EnvironmentID)
			case "changed-generation":
				lease.InstanceID, _ = core.NewEnvironmentInstanceID()
			case "ordinary-lease":
				lease.Ephemeral = false
			case "changed-workspace":
				lease.SourcePath += "-other"
			}
			data.Leases[lease.EnvironmentID] = lease
			raw, err = json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewEnvironmentJSONStore(s.path).ListEphemeralRuns(ctx); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("corrupt ownership accepted", err)
			}
		})
	}
}

func TestSchema13RunMigrationNeverInventsOwnership(t *testing.T) {
	s, run, lease := runIdentityFixture(t)
	run.InstanceID = ""
	lease.Ephemeral = false
	data := environmentFileState{Version: 13, EphemeralRuns: map[string]core.EphemeralRun{run.EnvironmentID: run}, Leases: map[string]core.WorkspaceLease{lease.EnvironmentID: lease}}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	runs, err := s.ListEphemeralRuns(ctx)
	if err != nil || len(runs) != 1 || runs[0].InstanceID != "" {
		t.Fatal("legacy identity invented", runs, err)
	}
	run.State = core.EphemeralRunCleanupRequired
	if err := s.PutEphemeralRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetWorkspaceLease(ctx, lease.EnvironmentID)
	if err != nil || got != lease {
		t.Fatal("legacy lease changed", got, err)
	}
	raw, err = os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 14 || data.EphemeralRuns[run.EnvironmentID].InstanceID != "" {
		t.Fatal("migration lost recovery evidence")
	}
}
