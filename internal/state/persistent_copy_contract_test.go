package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func detachedCopyFixture(t *testing.T) (*EnvironmentJSONStore, core.PersistentResource, core.PersistentResource) {
	t.Helper()
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	source := core.PersistentResource{ID: "oci:source", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/source", State: "creating", CreatedAt: time.Now().UTC()}
	mustSnapshot(t, store.BeginPersistentResourceCreate(context.Background(), source))
	mustSnapshot(t, store.CommitPersistentResourceCreate(context.Background(), source))
	source.State = "ready"
	target := core.PersistentResource{ID: "oci:copy", Owner: strings.Repeat("b", 32), Kind: source.Kind, NativeRef: "pool/copy", State: "creating", CreatedAt: source.CreatedAt, CopySource: source.Ref()}
	return store, source, target
}

func TestPersistentCopyCompletionDoesNotReleaseReservationsUntilCommit(t *testing.T) {
	ctx := context.Background()
	store, source, target := detachedCopyFixture(t)
	mustSnapshot(t, store.BeginPersistentResourceCopy(ctx, source, target))
	for _, completed := range []bool{false, true} {
		if completed {
			mustSnapshot(t, store.MarkPersistentResourceCopyCompleted(ctx, target))
		}
		// A new catalog handle must observe both identities after process restart.
		store = NewEnvironmentJSONStore(store.path)
		held, err := store.GetPersistentResource(ctx, target.ID)
		mustSnapshot(t, err)
		want := target
		want.CopyCompleted = completed
		if held != want {
			t.Fatal("copy receipt lost", held, want)
		}
		if _, err := store.BeginPersistentResourceDelete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal("copy source released before commit", err)
		}
		if _, err := store.BeginPersistentResourceDeleteOwned(ctx, held); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal("owned deletion cancelled unfinished copy", err)
		}
		if !completed {
			if err := store.CommitPersistentResourceCreate(ctx, held); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("unknown copy completion committed", err)
			}
		} else {
			if err := store.CommitPersistentResourceCreate(ctx, target); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("stale pre-completion receipt committed", err)
			}
			mustSnapshot(t, store.CommitPersistentResourceCreate(ctx, held))
		}
	}
	store = NewEnvironmentJSONStore(store.path)
	ready, err := store.GetPersistentResource(ctx, target.ID)
	mustSnapshot(t, err)
	want := target
	want.State, want.CopyCompleted, want.CopySource = "ready", false, core.PersistentResourceRef{}
	if ready != want {
		t.Fatal("committed copy lost independent identity", ready)
	}
	deleting, err := store.BeginPersistentResourceDeleteOwned(ctx, source)
	mustSnapshot(t, err)
	mustSnapshot(t, store.FinalizePersistentResourceDelete(ctx, deleting))
	if retained, err := NewEnvironmentJSONStore(store.path).GetPersistentResource(ctx, target.ID); err != nil || retained != ready {
		t.Fatal("source deletion affected independent copy", retained, err)
	}
}

func TestPersistentCopyRejectsForeignAndUnconfirmedReceiptsWithoutWriting(t *testing.T) {
	ctx := context.Background()
	store, source, target := detachedCopyFixture(t)
	mustSnapshot(t, store.BeginPersistentResourceCopy(ctx, source, target))
	for _, tc := range []struct {
		name string
		edit func(*core.PersistentResource)
		want error
	}{
		{"already-completed", func(r *core.PersistentResource) { r.CopyCompleted = true }, core.ErrInvalidArgument},
		{"missing-source", func(r *core.PersistentResource) { r.CopySource = core.PersistentResourceRef{} }, core.ErrInvalidArgument},
		{"already-ready", func(r *core.PersistentResource) { r.State = "ready" }, core.ErrInvalidArgument},
		{"foreign-owner", func(r *core.PersistentResource) { r.Owner = strings.Repeat("f", 32) }, core.ErrIncompatibleState},
		{"different-native-volume", func(r *core.PersistentResource) { r.NativeRef = "pool/foreign" }, core.ErrIncompatibleState},
		{"missing-target", func(r *core.PersistentResource) { r.ID = "oci:absent" }, core.ErrIncompatibleState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := os.ReadFile(store.path)
			mustSnapshot(t, err)
			changed := target
			tc.edit(&changed)
			if err := store.MarkPersistentResourceCopyCompleted(ctx, changed); !errors.Is(err, tc.want) {
				t.Fatal("invalid completion accepted", err)
			}
			after, err := os.ReadFile(store.path)
			mustSnapshot(t, err)
			if string(after) != string(before) {
				t.Fatal("rejected receipt changed durable catalog")
			}
		})
	}
}

func TestPersistentCopyAndAttachmentCompeteAcrossCatalogHandles(t *testing.T) {
	ctx := context.Background()
	store, source, target := detachedCopyFixture(t)
	other := NewEnvironmentJSONStore(store.path)
	lease := core.WorkspaceLease{EnvironmentID: "consumer", Owner: "consumer", WorkspaceID: "work", SourcePath: "/work", State: core.WorkspaceLeaseAcquiring, AccessMode: core.WorkspaceReadWrite, AcquiredAt: source.CreatedAt, PersistentResource: source.Ref()}
	start := make(chan struct{})
	var results [2]error
	var operations sync.WaitGroup
	operations.Go(func() { <-start; results[0] = store.BeginPersistentResourceCopy(ctx, source, target) })
	operations.Go(func() { <-start; results[1] = other.BeginEnvironmentCreate(ctx, lease) })
	close(start)
	operations.Wait()
	if (results[0] == nil) == (results[1] == nil) {
		t.Fatal("copy and writable attachment did not exclude each other", results)
	}
	store = NewEnvironmentJSONStore(store.path)
	if results[0] == nil {
		if !errors.Is(results[1], core.ErrStorageBusy) {
			t.Fatal(results)
		}
		if held, err := store.GetPersistentResource(ctx, target.ID); err != nil || held != target {
			t.Fatal("winning copy reservation lost", held, err)
		}
		if _, err := store.GetWorkspaceLease(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("rejected attachment partially reserved", err)
		}
	} else {
		if !errors.Is(results[0], core.ErrStorageBusy) {
			t.Fatal(results)
		}
		if held, err := store.GetWorkspaceLease(ctx, lease.EnvironmentID); err != nil || !held.Equal(lease) {
			t.Fatal("winning lease lost", held, err)
		}
		if _, err := store.GetPersistentResource(ctx, target.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("rejected copy partially reserved", err)
		}
	}
}

func TestWorkspaceResourceDeletionRequiresMatchingAssociation(t *testing.T) {
	ctx := context.Background()
	for _, sourceOnly := range []bool{false, true} {
		store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
		resource := core.PersistentResource{ID: "oci:bound", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/bound", State: "creating", CreatedAt: time.Now().UTC(), SourceOnly: sourceOnly}
		if !sourceOnly {
			resource.WorkspaceID = "owned-work"
		}
		mustSnapshot(t, store.BeginPersistentResourceCreate(ctx, resource))
		mustSnapshot(t, store.CommitPersistentResourceCreate(ctx, resource))
		resource.State = "ready"
		for _, workspace := range []core.WorkspaceID{"", "foreign-work"} {
			want := core.ErrIncompatibleState
			if workspace == "" {
				want = core.ErrInvalidArgument
			}
			if _, err := store.BeginWorkspaceResourceDelete(ctx, resource.ID, workspace); !errors.Is(err, want) {
				t.Fatal("foreign Workspace deleted retained data", workspace, err)
			}
		}
		if sourceOnly {
			if _, err := store.BeginWorkspaceResourceDelete(ctx, resource.ID, "owned-work"); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("Workspace adopted publication for deletion", err)
			}
			if held, err := NewEnvironmentJSONStore(store.path).GetPersistentResource(ctx, resource.ID); err != nil || held != resource {
				t.Fatal("publication changed", held, err)
			}
			continue
		}
		deleting, err := store.BeginWorkspaceResourceDelete(ctx, resource.ID, "owned-work")
		mustSnapshot(t, err)
		store = NewEnvironmentJSONStore(store.path)
		foreign := deleting
		foreign.Owner = strings.Repeat("f", 32)
		if _, err := store.BeginPersistentResourceDeleteOwned(ctx, foreign); !errors.Is(err, core.ErrCapabilityStale) {
			t.Fatal("foreign owner adopted deletion", err)
		}
		if _, err := store.BeginPersistentResourceDeleteOwned(ctx, core.PersistentResource{}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal("empty deletion authority accepted", err)
		}
		if held, err := store.GetPersistentResource(ctx, resource.ID); err != nil || held != deleting {
			t.Fatal("deletion identity changed across reopen", held, err)
		}
		mustSnapshot(t, store.FinalizePersistentResourceDelete(ctx, deleting))
		if _, err := store.GetPersistentResource(ctx, resource.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("confirmed absence did not release catalog", err)
		}
	}
}
