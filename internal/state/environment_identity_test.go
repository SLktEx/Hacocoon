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

func identityFixture(t *testing.T, s *EnvironmentJSONStore, instance string) core.Environment {
	t.Helper()
	ctx := context.Background()
	lease := core.WorkspaceLease{InstanceID: instance, EnvironmentID: "dev", WorkspaceID: "work", SourcePath: "/workspace/work", AccessMode: core.WorkspaceReadWrite, Owner: "dev", State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = "haco-dev"
	if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: "dev", Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, RuntimeRef: lease.RuntimeRef, CreatedAt: lease.AcquiredAt}
	if err := s.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	return env
}
func TestEnvironmentInstanceSurvivesReloadButNotRecreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := NewEnvironmentJSONStore(path)
	env := identityFixture(t, s, "") // pre-identity catalog
	id, err := s.EnvironmentInstance(context.Background(), env)
	if err != nil || !core.ValidEnvironmentInstanceID(id) {
		t.Fatalf("identity: %q %v", id, err)
	}
	reloaded := NewEnvironmentJSONStore(path)
	again, err := reloaded.EnvironmentInstance(context.Background(), env)
	if err != nil || again != id {
		t.Fatal("identity changed on reload")
	}
	lease, err := s.GetWorkspaceLease(context.Background(), "dev")
	if err != nil || lease.InstanceID != id {
		t.Fatal("identity was not persisted in lease")
	}
	if err := s.FinalizeEnvironmentDelete(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	recreated := identityFixture(t, s, "")
	next, err := s.EnvironmentInstance(context.Background(), recreated)
	if err != nil || next == id {
		t.Fatal("recreated name inherited old identity")
	}
	if _, err := s.EnvironmentInstance(context.Background(), env); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("stale snapshot accepted: %v", err)
	}
}
func TestEnvironmentInstanceCannotChangeInRuntimeReservation(t *testing.T) {
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	id, _ := core.NewEnvironmentInstanceID()
	lease := core.WorkspaceLease{InstanceID: id, EnvironmentID: "dev", WorkspaceID: "work", SourcePath: "/workspace/work", AccessMode: core.WorkspaceReadWrite, Owner: "dev", State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := s.BeginEnvironmentCreate(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	lease.InstanceID, _ = core.NewEnvironmentInstanceID()
	lease.RuntimeRef = "haco-dev"
	if err := s.RecordEnvironmentRuntime(context.Background(), lease); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("identity replacement accepted: %v", err)
	}
}

func TestEnvironmentInstanceDoesNotAdoptSynthesizedLease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewEnvironmentJSONStore(path)
	environment := identityFixture(t, store, "")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		t.Fatal(err)
	}
	delete(data, "workspace_leases")
	content, err = json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnvironmentInstance(context.Background(), environment); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("adopted missing lease: %v", err)
	}
}
