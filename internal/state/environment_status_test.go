package state

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestReadyEnvironmentObservesLifecycleWithoutWriting(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	st := NewEnvironmentJSONStore(path)
	if _, err := st.GetReadyEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	lease := core.WorkspaceLease{EnvironmentID: "demo", WorkspaceID: "work", SourcePath: "/work", Owner: "demo", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := st.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	check := func(want error) {
		t.Helper()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = st.GetReadyEnvironment(ctx, "demo")
		if !errors.Is(err, want) {
			t.Fatalf("status=%v want=%v", err, want)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("observation wrote catalog: %v", err)
		}
	}
	check(core.ErrRecoveryRequired)
	lease.RuntimeRef = "haco-demo"
	if err := st.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	check(core.ErrRecoveryRequired)
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: "demo", RuntimeRef: lease.RuntimeRef, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, CreatedAt: lease.AcquiredAt}
	if err := st.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	check(nil)
	lease.State = core.WorkspaceLeaseCleanupRequired
	if err := st.MarkEnvironmentRecoveryRequired(ctx, lease); err != nil {
		t.Fatal(err)
	}
	check(core.ErrRecoveryRequired)
	if got, err := st.GetEnvironment(ctx, "demo"); err != nil || got != env {
		t.Fatalf("cleanup cannot find retained metadata: %#v %v", got, err)
	}
	if err := st.FinalizeEnvironmentDelete(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	check(core.ErrNotFound)
}

func TestReadyEnvironmentRejectsMismatchedLease(t *testing.T) {
	ctx := context.Background()
	st := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	env := core.Environment{Name: "demo", RuntimeRef: "haco-demo", Workspace: core.Workspace{ID: "work", Path: "/work"}, AccessMode: core.WorkspaceReadWrite, CreatedAt: time.Now().UTC()}
	data := newEnvironmentFileState()
	data.Environments["demo"] = env
	data.Leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", RuntimeRef: "other", WorkspaceID: "work", SourcePath: "/work", AccessMode: env.AccessMode, State: core.WorkspaceLeaseActive, AcquiredAt: env.CreatedAt}
	if err := st.writeEnvironments(data); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetReadyEnvironment(ctx, "demo"); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
}
