package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
	"time"
)

type managedDeleteProvider struct {
	work    core.Workspace
	calls   int
	resolve int
	drift   bool
}

func (p *managedDeleteProvider) Resolve(context.Context, WorkspaceRequest) (core.Workspace, error) {
	p.resolve++
	w := p.work
	if p.drift && p.resolve > 1 {
		w.ID += "changed"
	}
	return w, nil
}
func (p *managedDeleteProvider) DeleteWorkspace(_ context.Context, w core.Workspace) error {
	if w != p.work {
		return core.ErrCapabilityStale
	}
	p.calls++
	return nil
}
func (p *managedDeleteProvider) ListManagedWorkspaces(context.Context) ([]ManagedWorkspace, error) {
	return []ManagedWorkspace{{Workspace: p.work, Name: "work", State: "ready"}}, nil
}
func TestManagedWorkspaceDeletionUsesLifecycleLockAndRejectsLease(t *testing.T) {
	ctx := context.Background()
	work := core.Workspace{ID: "workspace:managed:owned", Path: "managed:work"}
	p := &managedDeleteProvider{work: work}
	catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	s := NewWithProvider(nil, catalog, p)
	unlock, err := lockWorkspace(ctx, work.ID)
	if err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := s.DeleteManagedWorkspace(bounded, work); !errors.Is(err, context.DeadlineExceeded) || p.calls != 0 {
		t.Fatal(err)
	}
	unlock()
	id, _ := core.NewEnvironmentInstanceID()
	l := core.WorkspaceLease{InstanceID: id, EnvironmentID: "dev", WorkspaceID: work.ID, SourcePath: work.Path, AccessMode: core.WorkspaceReadWrite, Owner: "dev", State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := catalog.BeginEnvironmentCreate(ctx, l); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteManagedWorkspace(ctx, work); !errors.Is(err, core.ErrStorageBusy) || p.calls != 0 {
		t.Fatal(err)
	}
	all, err := s.ListManagedWorkspaces(ctx)
	if err != nil || len(all) != 1 || len(all[0].Environments) != 1 || all[0].Environments[0] != "dev" {
		t.Fatal(all, err)
	}
	if err := catalog.FinalizeEnvironmentDelete(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteManagedWorkspace(ctx, work); err != nil || p.calls != 1 {
		t.Fatal(err)
	}
}
func TestCreateRechecksWorkspaceIdentityAfterLifecycleLock(t *testing.T) {
	p := &managedDeleteProvider{work: core.Workspace{ID: "workspace:managed:old", Path: "managed:work"}, drift: true}
	s := NewWithProvider(nil, state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json")), p)
	_, err := s.Create(context.Background(), core.EnvironmentSpec{Name: "dev", WorkspacePath: "managed:work", SkipDefaultResource: true})
	if !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("accepted Workspace replaced after resolution", err)
	}
}

func TestCreatePinsReviewedWorkspaceBeforeAnyProviderMutation(t *testing.T) {
	p := &managedDeleteProvider{work: core.Workspace{ID: "workspace:managed:new", Path: "managed:work"}}
	// A nil runtime intentionally makes any attempt to mutate the provider fail.
	s := NewWithProvider(nil, state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json")), p)
	_, err := s.Create(context.Background(), core.EnvironmentSpec{Name: "dev", WorkspacePath: "managed:work", ExpectedWorkspace: "workspace:managed:old"})
	if !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("adopted recycled Workspace", err)
	}
}
