package persistentresource_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type restoreBackend struct {
	backend
	t     *testing.T
	saved core.Snapshot
}

func (b *restoreBackend) PlanSavedResource(context.Context, core.Snapshot, string) (string, string, error) {
	return "oci-containerd", "pool/owned-restored", nil
}
func (b *restoreBackend) CreateSavedResource(ctx context.Context, _ core.Snapshot, r core.PersistentResource) error {
	b.t.Helper()
	record, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || record != r {
		b.t.Fatal("copy before reservation", err)
	}
	if _, err := b.store.BeginPersistentResourceDelete(ctx, r.ID); !errors.Is(err, core.ErrRecoveryRequired) {
		b.t.Fatal("ordinary delete raced active restore", err)
	}
	if err := b.store.BeginSnapshotDelete(ctx, b.saved.ID); !errors.Is(err, core.ErrStorageBusy) {
		b.t.Fatal("saved copy not reserved", err)
	}
	if err := b.store.CommitPersistentResourceCreate(ctx, r); !errors.Is(err, core.ErrRecoveryRequired) {
		b.t.Fatal("published without creation receipt", err)
	}
	b.exists = true
	if b.fail == "copy" || b.fail == "delete" {
		return core.ErrRecoveryRequired
	}
	return nil
}
func (b *restoreBackend) Verify(ctx context.Context, r core.PersistentResource) error {
	record, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || record != r || r.State != "created" {
		b.t.Fatal("verification before receipt", record, err)
	}
	if _, err := b.store.BeginPersistentResourceDelete(ctx, r.ID); !errors.Is(err, core.ErrRecoveryRequired) {
		b.t.Fatal("ordinary delete raced restore verification", err)
	}
	if b.fail == "verify" {
		return core.ErrCapabilityStale
	}
	return nil
}
func restoreFixture(t *testing.T) (*state.EnvironmentJSONStore, core.Snapshot) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	saved := core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready", Source: core.SnapshotSource{InstanceID: "env-" + strings.Repeat("b", 32), Environment: core.Environment{Name: "old", Workspace: core.Workspace{ID: "work", Path: "managed:work"}, RuntimeRef: "haco-old", AccessMode: core.WorkspaceReadWrite, CreatedAt: time.Now().UTC(), PersistentResource: core.PersistentResourceRef{ID: "oci:old", Owner: strings.Repeat("c", 32)}}}}
	for i, role := range []string{"rootfs", "workspace:one", "oci"} {
		saved.Components = append(saved.Components, core.SnapshotComponent{Role: role, NativeRef: "native-" + role, Owner: strings.Repeat(string(rune('d'+i)), 32), State: "verified"})
	}
	raw, err := json.Marshal(map[string]any{"version": 10, "snapshots": map[string]core.Snapshot{saved.ID: saved}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return state.NewEnvironmentJSONStore(path), saved
}
func TestSnapshotStoreReservationReceiptAndFailureCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "copy", "verify", "delete"} {
		t.Run(mode, func(t *testing.T) {
			store, saved := restoreFixture(t)
			b := &restoreBackend{backend: backend{store: store, fail: mode}, t: t, saved: saved}
			service := persistentresource.Service{Store: store, Backend: b}
			result, err := service.RestoreSnapshot(context.Background(), "oci:restored", saved, "new-work")
			if mode == "ok" {
				if err != nil || result.State != "ready" || result.RestoreSource != "" || result.WorkspaceID != "new-work" {
					t.Fatal(result, err)
				}
				if err := store.BeginSnapshotDelete(context.Background(), saved.ID); err != nil {
					t.Fatal("publication kept source reservation", err)
				}
			} else {
				if err == nil {
					t.Fatal("failed copy published")
				}
				if mode == "delete" {
					if !errors.Is(err, core.ErrRecoveryRequired) {
						t.Fatal(err)
					}
					held, err := store.GetPersistentResource(context.Background(), "oci:restored")
					if err != nil || held.RestoreSource != saved.ID || held.State != "deleting" {
						t.Fatal("lost residue", held, err)
					}
					if err := store.BeginSnapshotDelete(context.Background(), saved.ID); !errors.Is(err, core.ErrStorageBusy) {
						t.Fatal("released source prematurely", err)
					}
					foreign := held
					foreign.Owner = strings.Repeat("f", 32)
					if _, err := store.BeginPersistentResourceDeleteOwned(context.Background(), foreign); !errors.Is(err, core.ErrCapabilityStale) {
						t.Fatal("foreign cleanup accepted", err)
					}
					b.fail = ""
					if err := service.Delete(context.Background(), held.ID); err != nil {
						t.Fatal(err)
					}
				} else if errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("cleaned failure marked recovery", err)
				}
				if _, err := store.GetPersistentResource(context.Background(), "oci:restored"); !errors.Is(err, core.ErrNotFound) {
					t.Fatal("cleanup record remains", err)
				}
				if err := store.BeginSnapshotDelete(context.Background(), saved.ID); err != nil {
					t.Fatal("cleanup kept source reservation", err)
				}
			}
		})
	}
}
