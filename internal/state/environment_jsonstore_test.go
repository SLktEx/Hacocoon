package state

import (
	"context"
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

func TestEnvironmentJSONStoreRejectsV01State(t *testing.T) {
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
	if _, err := store.GetEnvironment(context.Background(), "old"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("legacy state error = %v", err)
	}
}
