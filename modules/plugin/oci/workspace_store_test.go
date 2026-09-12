package oci

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

type workspaceCopyBackend struct {
	store   persistentresource.Store
	copies  int
	failure error
}

func (*workspaceCopyBackend) Plan(_ context.Context, kind, owner string) (string, error) {
	return "pool/" + owner, nil
}
func (*workspaceCopyBackend) Create(context.Context, core.PersistentResource) error { return nil }
func (*workspaceCopyBackend) Verify(context.Context, core.PersistentResource) error { return nil }
func (*workspaceCopyBackend) Delete(context.Context, core.PersistentResource) error { return nil }
func (b *workspaceCopyBackend) Copy(ctx context.Context, src, dst core.PersistentResource) error {
	b.copies++
	persisted, err := b.store.GetPersistentResource(ctx, dst.ID)
	if err != nil {
		return err
	}
	if persisted.WorkspaceID == "" || persisted.WorkspaceID != dst.WorkspaceID || persisted.CopySource != src.Ref() {
		return errors.New("copy preceded durable association")
	}
	return b.failure
}
func TestDefaultWorkspaceCopyReusesExactPersistedAssociation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	st := state.NewEnvironmentJSONStore(path)
	backend := &workspaceCopyBackend{store: st}
	svc := &persistentresource.Service{Store: st, Backend: backend}
	resolver := WorkspaceStores{Resources: svc}
	work := core.Workspace{ID: "managed:first", Path: "managed:first"}
	empty, err := resolver.Resolve(ctx, work)
	if err != nil || empty != (core.PersistentResource{}) || backend.copies != 0 {
		t.Fatalf("no source: %+v %v", empty, err)
	}
	if _, err := svc.PublishSource(ctx, HostStoreID, StoreKind, func(context.Context, core.PersistentResource) error { return nil }); err != nil {
		t.Fatal(err)
	}
	first, err := resolver.Resolve(ctx, work)
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkspaceID != work.ID || backend.copies != 1 {
		t.Fatal("not automatically copied")
	}
	// Simulate restart: re-read durable catalog, not a client memory cache.
	svc.Store = state.NewEnvironmentJSONStore(path)
	again, err := resolver.Resolve(ctx, work)
	if err != nil || again.Ref() != first.Ref() || backend.copies != 1 {
		t.Fatalf("recreate lost retained store: %+v %v", again, err)
	}
	second, err := resolver.Resolve(ctx, core.Workspace{ID: "managed:second", Path: "managed:second"})
	if err != nil || second.Ref() == first.Ref() || backend.copies != 2 {
		t.Fatalf("workspaces share a copy: %+v %v", second, err)
	}
}
func TestFailedAutomaticCopyDoesNotRetryOrPublishEmptyStore(t *testing.T) {
	ctx := context.Background()
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	backend := &workspaceCopyBackend{store: st, failure: errors.New("lost completion")}
	svc := &persistentresource.Service{Store: st, Backend: backend}
	if _, err := svc.PublishSource(ctx, HostStoreID, StoreKind, func(context.Context, core.PersistentResource) error { return nil }); err != nil {
		t.Fatal(err)
	}
	resolver := WorkspaceStores{Resources: svc}
	work := core.Workspace{ID: "managed:dev"}
	for i := 0; i < 2; i++ {
		if _, err := resolver.Resolve(ctx, work); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
	}
	if backend.copies != 1 {
		t.Fatal("uncertain copy retried")
	}
	list, err := st.ListPersistentResources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range list {
		if r.WorkspaceID == work.ID {
			found = true
			if r.State != "creating" || r.CopySource.ID != HostStoreID {
				t.Fatalf("lost recovery evidence: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("automatic resource orphaned")
	}
}

func TestPublicationIsNotCopyableOrDeletableUntilContentPreparationCompletes(t *testing.T) {
	ctx := context.Background()
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	backend := &workspaceCopyBackend{store: st}
	svc := &persistentresource.Service{Store: st, Backend: backend}
	_, err := svc.PublishSource(ctx, HostStoreID, StoreKind, func(ctx context.Context, source core.PersistentResource) error {
		saved, err := st.GetPersistentResource(ctx, source.ID)
		if err != nil {
			return err
		}
		if !saved.SourceOnly || saved.State != "creating" || saved.Ref() != source.Ref() {
			t.Fatal("publication lacks durable reservation")
		}
		if _, err := (WorkspaceStores{Resources: svc}).Resolve(ctx, core.Workspace{ID: "work"}); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatalf("partial content copied: %v", err)
		}
		if err := svc.Delete(ctx, source.ID); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatalf("active publication deleted: %v", err)
		}
		return errors.New("publisher crashed")
	})
	if !errors.Is(err, core.ErrRecoveryRequired) || backend.copies != 0 {
		t.Fatalf("%v copies=%d", err, backend.copies)
	}
}

func TestTemporaryCopyCleanupChecksWorkspaceOwnershipAndKeepsSource(t *testing.T) {
	ctx := context.Background()
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	backend := &workspaceCopyBackend{store: st}
	resources := &persistentresource.Service{Store: st, Backend: backend}
	resolver := WorkspaceStores{Resources: resources}
	if _, err := resources.PublishSource(ctx, HostStoreID, StoreKind, func(context.Context, core.PersistentResource) error { return nil }); err != nil {
		t.Fatal(err)
	}
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	copied, err := resolver.Resolve(ctx, work)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.DeleteForWorkspace(ctx, copied.ID, "different-workspace"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	if err := resolver.CleanupTemporary(ctx, work); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPersistentResource(ctx, copied.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("temporary copy retained")
	}
	if _, err := st.GetPersistentResource(ctx, HostStoreID); err != nil {
		t.Fatal("source removed")
	}
	if err := resolver.CleanupTemporary(ctx, work); err != nil {
		t.Fatal("cleanup not retryable", err)
	}
	if err := resolver.CleanupTemporary(ctx, core.Workspace{ID: "regular", Path: "/work"}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}

func TestRetainedExplicitStoreWinsOverHostPublicationAfterRecreation(t *testing.T) {
	ctx := context.Background()
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &workspaceCopyBackend{store: st}
	svc := &persistentresource.Service{Store: st, Backend: b}
	if _, err := svc.PublishSource(ctx, HostStoreID, StoreKind, func(context.Context, core.PersistentResource) error { return nil }); err != nil {
		t.Fatal(err)
	}
	work := core.Workspace{ID: "managed:fork"}
	retained, err := svc.CopyForWorkspace(ctx, "oci:fork", StoreKind, HostStoreID, work.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := (WorkspaceStores{Resources: svc}).Resolve(ctx, work)
	if err != nil || got.Ref() != retained.Ref() || b.copies != 1 {
		t.Fatal(got, err, b.copies)
	}
	if _, err = svc.CopyForWorkspace(ctx, "oci:second", StoreKind, HostStoreID, work.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = (WorkspaceStores{Resources: svc}).Resolve(ctx, work); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("ambiguous association accepted", err)
	}
}
