package persistentresource_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type savedDataBackend struct {
	t      *testing.T
	store  *state.EnvironmentJSONStore
	fail   string
	copies int
}

func (b *savedDataBackend) Plan(_ context.Context, _, owner string) (string, error) {
	return "pool/" + owner, nil
}
func (b *savedDataBackend) Create(context.Context, core.PersistentResource) error {
	b.t.Fatal("saved bytes replaced by empty data")
	return nil
}
func (b *savedDataBackend) Verify(ctx context.Context, r core.PersistentResource) error {
	current, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || current.State != "created" {
		b.t.Fatal("verify before completion receipt", err)
	}
	if b.fail == "verify" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (b *savedDataBackend) Delete(ctx context.Context, r core.PersistentResource) error {
	current, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || current != r || r.State != "deleting" {
		b.t.Fatal("delete without reservation", err)
	}
	if b.fail == "delete" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (b *savedDataBackend) PlanSavedEnvironmentResource(_ context.Context, _ core.Snapshot, _, owner string) (string, string, error) {
	return "build-cache", "pool/" + owner, nil
}
func (b *savedDataBackend) CreateSavedEnvironmentResource(ctx context.Context, saved core.Snapshot, key string, r core.PersistentResource) error {
	b.copies++
	current, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || current != r || r.RestoreSource != saved.ID {
		b.t.Fatal("copy without reserved source", err)
	}
	if err := b.store.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
		b.t.Fatal("snapshot deleted during copy", err)
	}
	if b.fail == "copy" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func savedDataFixture(t *testing.T) (*state.EnvironmentJSONStore, core.Snapshot) {
	t.Helper()
	ctx := context.Background()
	manager, backend, request, selections := environmentContractService(t)
	request.Workspace = core.Workspace{ID: "source-work", Path: "managed:source"}
	store := backend.store
	lease, _ := reserveEnvironmentContract(t, manager, store, request, selections)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := manager.MaterializeEnvironmentResources(ctx, lease)
	must(err)
	lease.RuntimeRef = "haco-source"
	must(store.RecordEnvironmentRuntime(ctx, lease))
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: lease.EnvironmentID, Workspace: request.Workspace, RuntimeRef: lease.RuntimeRef, AccessMode: lease.AccessMode, Attachments: lease.Attachments, CreatedAt: lease.AcquiredAt}
	must(store.CommitEnvironmentCreate(ctx, env, lease))
	saved := core.Snapshot{ID: "snap-" + strings.Repeat("7", 32), State: "capturing", Source: core.SnapshotSource{Environment: env, InstanceID: lease.InstanceID}}
	roles := []string{"rootfs", "workspace:main"}
	for _, area := range lease.Attachments {
		roles = append(roles, "data:"+area.Key)
	}
	for i, role := range roles {
		saved.Components = append(saved.Components, core.SnapshotComponent{Role: role, NativeRef: "saved/" + role, Owner: fmt.Sprintf("%032x", 100+i), State: "planned"})
	}
	must(store.BeginSnapshot(ctx, saved))
	for _, c := range saved.Components {
		must(store.RecordSnapshotComponent(ctx, saved.ID, c, "created"))
		c.State = "created"
		must(store.RecordSnapshotComponent(ctx, saved.ID, c, "verified"))
	}
	must(store.CommitSnapshot(ctx, saved.ID))
	saved, err = store.GetSnapshot(ctx, saved.ID)
	must(err)
	return store, saved
}

func TestSavedDataReservationReceiptAndCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "source-changed", "copy", "verify", "delete"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store, saved := savedDataFixture(t)
			if mode == "source-changed" {
				for _, a := range saved.Source.Environment.Attachments {
					if _, err := store.ResetResourceGeneration(ctx, a.Origin, a.Origin.Compatibility); err != nil {
						t.Fatal(err)
					}
				}
			}
			backend := &savedDataBackend{t: t, store: store, fail: mode}
			manager := &persistentresource.Service{Store: store, Backend: backend}
			lease := core.WorkspaceLease{EnvironmentID: "restored", Owner: "restored", InstanceID: "env-" + strings.Repeat("9", 32), WorkspaceID: "new-work", SourcePath: "managed:new", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), SnapshotSource: saved.ID}
			plans, err := manager.PlanSavedEnvironmentResources(ctx, core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}}, saved)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range plans {
				lease.Attachments = append(lease.Attachments, p.Attachment)
			}
			err = store.BeginEnvironmentCreateFromSnapshotWithResources(ctx, lease, saved, plans)
			if err != nil {
				t.Fatal(err)
			}
			areas, err := manager.MaterializeEnvironmentResources(ctx, lease)
			if mode == "copy" || mode == "verify" {
				if err == nil {
					t.Fatal("failed copy published")
				}
				if err := store.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
					t.Fatal("failed copy source released", err)
				}
			} else if err != nil || len(areas) != 2 || areas[0].Resource.RestoreSource != "" {
				t.Fatal("saved data did not publish", areas, err)
			}
			absent, err := store.PrepareEnvironmentResourceDeletion(ctx, lease)
			if err != nil {
				t.Fatal(err)
			}
			err = manager.DeleteEnvironmentResources(ctx, absent)
			if mode == "copy" || mode == "delete" {
				if err == nil {
					t.Fatal("unknown provider state treated as absent")
				}
				if err := store.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("lost parent", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := store.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); err != nil {
					t.Fatal(err)
				}
				if err := store.BeginSnapshotDelete(ctx, saved.ID); err != nil {
					t.Fatal("completed cleanup kept source pinned", err)
				}
			}
		})
	}
}
