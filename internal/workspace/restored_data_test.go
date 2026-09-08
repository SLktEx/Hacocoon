package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type restoredDataProvider struct{ work core.Workspace }

func (p restoredDataProvider) Resolve(context.Context, WorkspaceRequest) (core.Workspace, error) {
	return p.work, nil
}
func TestRestoredDataCleanupExcludesLeasesAndIdentityDrift(t *testing.T) {
	ctx := context.Background()
	work := core.Workspace{ID: "workspace:managed:owned", Path: "managed:restored"}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	s := NewWithProvider(nil, store, restoredDataProvider{work})
	calls := 0
	remove := func(context.Context) error { calls++; return nil }
	if err := s.CleanupRestoredData(ctx, work, remove); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	id, _ := core.NewEnvironmentInstanceID()
	lease := core.WorkspaceLease{InstanceID: id, EnvironmentID: "dev", WorkspaceID: work.ID, SourcePath: work.Path, AccessMode: core.WorkspaceReadWrite, Owner: "dev", State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanupRestoredData(ctx, work, remove); !errors.Is(err, core.ErrStorageBusy) || calls != 1 {
		t.Fatal("acquiring lease ignored", err, calls)
	}
	changed := work
	changed.ID = "workspace:managed:foreign"
	s.provider = restoredDataProvider{changed}
	if err := s.CleanupRestoredData(ctx, work, remove); !errors.Is(err, core.ErrCapabilityStale) || calls != 1 {
		t.Fatal("reused registry name", err, calls)
	}
}
func TestRestoredDataCleanupUsesCanonicalWorkspaceLock(t *testing.T) {
	ctx := context.Background()
	work := core.Workspace{ID: "workspace:managed:lock", Path: "managed:restored"}
	s := NewWithProvider(nil, state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json")), restoredDataProvider{work})
	unlock, err := lockWorkspace(ctx, work.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	bounded, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := s.CleanupRestoredData(bounded, work, func(context.Context) error { t.Fatal("cleanup overtook lifecycle lock"); return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
