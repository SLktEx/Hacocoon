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

func TestEnvironmentJSONStoreRoundTripAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "environments.json")
	store := NewEnvironmentJSONStore(path)
	ctx := context.Background()
	environment := core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{ID: "workspace:demo", Path: "/tmp/workspace"},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "haco-demo",
		CreatedAt:  time.Date(2026, 8, 29, 6, 30, 0, 0, time.UTC),
	}

	if err := store.PutEnvironment(ctx, environment); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetEnvironment(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != environment {
		t.Fatalf("got %#v want %#v", got, environment)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := store.DeleteEnvironment(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("get after delete = %v", err)
	}
	if err := store.DeleteEnvironment(ctx, "demo"); err != nil {
		t.Fatalf("repeated delete must be idempotent: %v", err)
	}
}

func TestEnvironmentJSONStoreMissingIsNotFound(t *testing.T) {
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := store.GetEnvironment(context.Background(), "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnvironmentJSONStoreReadsLegacyMetadataButRefusesToEnableLeases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	legacy := `{
  "environments": {
    "old": {
      "name": "old",
      "runtime_ref": "haco-old",
      "created_at": "2026-08-29T00:00:00Z"
    }
  }
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewEnvironmentJSONStore(path)
	if _, err := store.GetEnvironment(context.Background(), "old"); err != nil {
		t.Fatalf("legacy metadata must remain readable for existing client operations: %v", err)
	}
	err := store.AcquireWorkspaceLease(context.Background(), core.WorkspaceLease{
		WorkspaceID:   "workspace:new",
		EnvironmentID: "new",
		AccessMode:    core.WorkspaceReadWrite,
	})
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("legacy state lease error = %v", err)
	}
}

func TestEnvironmentJSONStoreEphemeralRunRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "environments.json")
	store := NewEnvironmentJSONStore(path)
	ctx := context.Background()
	run := core.EphemeralRun{
		EnvironmentID: "run-deadbeef",
		State:         core.EphemeralRunActive,
		CreatedAt:     time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC),
	}
	if err := store.PutEphemeralRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	runs, err := store.ListEphemeralRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0] != run {
		t.Fatalf("runs=%#v", runs)
	}
	if err := store.DeleteEphemeralRun(ctx, run.EnvironmentID); err != nil {
		t.Fatal(err)
	}
	runs, err = store.ListEphemeralRuns(ctx)
	if err != nil || len(runs) != 0 {
		t.Fatalf("runs after delete=%#v err=%v", runs, err)
	}
}

func TestEnvironmentJSONStoreMigratesVersion2WhenWritingEphemeralMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"environments":{},"workspace_leases":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewEnvironmentJSONStore(path)
	if err := store.PutEphemeralRun(context.Background(), core.EphemeralRun{
		EnvironmentID: "run-migrate",
		State:         core.EphemeralRunCreating,
		CreatedAt:     time.Date(2026, 8, 30, 4, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted struct {
		Version       int                          `json:"version"`
		EphemeralRuns map[string]core.EphemeralRun `json:"ephemeral_runs"`
	}
	if err := json.Unmarshal(contents, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Version != environmentStateVersion {
		t.Fatalf("version=%d", persisted.Version)
	}
	if persisted.EphemeralRuns["run-migrate"].EnvironmentID != "run-migrate" {
		t.Fatalf("state=%s", contents)
	}
}
