package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestOwnedWorkspaceCleanupPersistsUntilExactFinalization(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "environments.json")
	s := NewEnvironmentJSONStore(path)
	lease := core.WorkspaceLease{WorkspaceID: "workspace:managed:owned", SourcePath: "managed:data", EnvironmentID: "work", Owner: "work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = "haco-work"
	if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	env := core.Environment{Name: "work", RuntimeRef: lease.RuntimeRef, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, OwnedWorkspace: true, CreatedAt: lease.AcquiredAt}
	lease.State = core.WorkspaceLeaseActive
	if err := s.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.RememberOpened(ctx, env); err != nil {
		t.Fatal(err)
	}
	if last, err := s.LastOpened(ctx); err != nil || last != "work" {
		t.Fatal(last, err)
	}
	stale := env
	stale.RuntimeRef = "replacement"
	if err := s.RememberOpened(ctx, stale); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if err := s.SetDefaultImage(ctx, "", false); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if err := s.FinalizeEnvironmentDelete(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	s = NewEnvironmentJSONStore(path)
	if last, err := s.LastOpened(ctx); err != nil || last != "" {
		t.Fatal("deleted continuation retained", last, err)
	}
	work, err := s.OwnedWorkspaceCleanup(ctx, "work")
	if err != nil || work != env.Workspace {
		t.Fatal("pending cleanup lost", work, err)
	}
	wrong := work
	wrong.ID = "workspace:managed:replacement"
	if err := s.FinalizeOwnedWorkspaceCleanup(ctx, "work", wrong); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("replacement finalized", err)
	}
	reservation := lease
	reservation.State = core.WorkspaceLeaseAcquiring
	reservation.RuntimeRef = ""
	if err := s.BeginEnvironmentCreate(ctx, reservation); err == nil {
		t.Fatal("pending owner name reused")
	}
	for i := 0; i < 2; i++ {
		if err := s.FinalizeOwnedWorkspaceCleanup(ctx, "work", work); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.OwnedWorkspaceCleanup(ctx, "work"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.RememberOpened(ctx, env); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("absent Environment remembered", err)
	}
}

func TestOwnedWorkspaceCleanupRejectsConflictingCatalogRelations(t *testing.T) {
	work := core.Workspace{ID: "workspace:managed:owned", Path: "managed:data"}
	for _, kind := range []string{"valid", "name", "id", "path", "duplicate", "environment", "lease-owner", "lease-workspace", "lease-path"} {
		t.Run(kind, func(t *testing.T) {
			data := newEnvironmentFileState()
			data.OwnedWorkspaceCleanup["work"] = work
			switch kind {
			case "name":
				delete(data.OwnedWorkspaceCleanup, "work")
				data.OwnedWorkspaceCleanup["../work"] = work
			case "id":
				v := work
				v.ID = "external"
				data.OwnedWorkspaceCleanup["work"] = v
			case "path":
				v := work
				v.Path = "/external"
				data.OwnedWorkspaceCleanup["work"] = v
			case "duplicate":
				data.OwnedWorkspaceCleanup["other"] = work
			case "environment":
				data.Environments["work"] = core.Environment{Name: "work"}
			case "lease-owner":
				data.Leases["work"] = core.WorkspaceLease{}
			case "lease-workspace":
				data.Leases["other"] = core.WorkspaceLease{WorkspaceID: work.ID}
			case "lease-path":
				data.Leases["other"] = core.WorkspaceLease{SourcePath: work.Path}
			}
			err := validateOwnedWorkspaceCleanup(data)
			if kind == "valid" && err != nil || kind != "valid" && !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(kind, err)
			}
		})
	}
}
