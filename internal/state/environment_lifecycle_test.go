package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEnvironmentLifecyclePublishesEnvironmentAndActiveLeaseAtomically(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	acquiredAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	lease := core.WorkspaceLease{
		WorkspaceID:   core.WorkspaceID("ws-demo"),
		SourcePath:    "/workspace/demo",
		EnvironmentID: "demo",
		AccessMode:    core.WorkspaceReadWrite,
		Owner:         "demo",
		State:         core.WorkspaceLeaseAcquiring,
		AcquiredAt:    acquiredAt,
	}
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}

	lease.RuntimeRef = "haco-demo"
	if err := store.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Environment published before ready commit: %v", err)
	}
	reserved, err := store.GetWorkspaceLease(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if reserved.State != core.WorkspaceLeaseAcquiring || reserved.RuntimeRef != "haco-demo" {
		t.Fatalf("runtime reservation = %#v", reserved)
	}

	environment := core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath},
		AccessMode: lease.AccessMode,
		RuntimeRef: lease.RuntimeRef,
		CreatedAt:  acquiredAt.Add(time.Second),
	}
	lease.State = core.WorkspaceLeaseActive
	if err := store.CommitEnvironmentCreate(ctx, environment, lease); err != nil {
		t.Fatal(err)
	}
	gotEnvironment, err := store.GetEnvironment(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	gotLease, err := store.GetWorkspaceLease(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if gotEnvironment.RuntimeRef != "haco-demo" || gotLease.State != core.WorkspaceLeaseActive || gotLease.RuntimeRef != gotEnvironment.RuntimeRef {
		t.Fatalf("committed environment=%#v lease=%#v", gotEnvironment, gotLease)
	}

	if err := store.FinalizeEnvironmentDelete(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Environment remains after finalize: %v", err)
	}
	if _, err := store.GetWorkspaceLease(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("lease remains after finalize: %v", err)
	}
}

func TestEnvironmentLifecycleRejectsReadyCommitBeforeRuntimeOwnershipIsRecorded(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	lease := core.WorkspaceLease{
		WorkspaceID:   core.WorkspaceID("ws-demo"),
		SourcePath:    "/workspace/demo",
		EnvironmentID: "demo",
		AccessMode:    core.WorkspaceReadWrite,
		Owner:         "demo",
		State:         core.WorkspaceLeaseAcquiring,
		AcquiredAt:    time.Now().UTC(),
	}
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}

	environment := core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath},
		AccessMode: lease.AccessMode,
		RuntimeRef: "haco-demo",
		CreatedAt:  time.Now().UTC(),
	}
	active := lease
	active.RuntimeRef = environment.RuntimeRef
	active.State = core.WorkspaceLeaseActive
	if err := store.CommitEnvironmentCreate(ctx, environment, active); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("commit error = %v", err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("invalid Environment was published: %v", err)
	}
	reserved, err := store.GetWorkspaceLease(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if reserved.State != core.WorkspaceLeaseAcquiring || reserved.RuntimeRef != "" {
		t.Fatalf("reservation mutated after rejected commit: %#v", reserved)
	}
}

func TestEnvironmentLifecycleRecoveryNeverForgetsRecordedRuntime(t *testing.T) {
	ctx := context.Background()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	lease := core.WorkspaceLease{
		WorkspaceID:   core.WorkspaceID("ws-demo"),
		SourcePath:    "/workspace/demo",
		EnvironmentID: "demo",
		AccessMode:    core.WorkspaceReadWrite,
		Owner:         "demo",
		State:         core.WorkspaceLeaseAcquiring,
		AcquiredAt:    time.Now().UTC(),
	}
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = "haco-demo"
	if err := store.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}

	recovery := lease
	recovery.RuntimeRef = ""
	recovery.State = core.WorkspaceLeaseCleanupRequired
	if err := store.MarkEnvironmentRecoveryRequired(ctx, recovery); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetWorkspaceLease(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != core.WorkspaceLeaseCleanupRequired || got.RuntimeRef != "haco-demo" {
		t.Fatalf("recovery lease = %#v", got)
	}
}
