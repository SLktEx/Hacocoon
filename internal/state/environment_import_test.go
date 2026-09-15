package state

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
	"io"
	"strings"
	"testing"
	"time"
)

type importedDataBackend struct{ *savedDataBackend }

func (b *importedDataBackend) Import(ctx context.Context, r core.PersistentResource, reader io.ReadSeeker) error {
	b.copies++
	current, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || current != r || !r.ImportPending || r.State != "creating" {
		b.t.Fatal("import before durable reservation", err)
	}
	bytes, err := io.ReadAll(reader)
	if err != nil || string(bytes) != "saved bytes" {
		b.t.Fatal("wrong import input", err)
	}
	if b.fail == "unknown" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}

func TestImportedDataRequiresBytesAndPositiveCompletion(t *testing.T) {
	for _, mode := range []string{"ok", "wrong-mode", "unknown", "verify", "delete"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store, _, _ := environmentDataFixture(t, 1)
			backend := &importedDataBackend{&savedDataBackend{t: t, store: store, fail: mode}}
			manager := &persistentresource.Service{Store: store, Backend: backend}
			lease := core.WorkspaceLease{EnvironmentID: "imported", Owner: "imported", InstanceID: "env-" + strings.Repeat("d", 32), WorkspaceID: "imported-work", SourcePath: "managed:imported", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
			inputs := []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: strings.Repeat("a", 64), Archive: strings.NewReader("saved bytes")}}
			plans, err := manager.PlanImportedEnvironmentResources(ctx, core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}}, inputs)
			if err != nil {
				t.Fatal(err)
			}
			lease.Attachments = []core.EnvironmentAttachment{plans[0].Attachment}
			if !strings.HasPrefix(lease.Attachments[0].Origin.Name, "import-") || !plans[0].Resource.ImportPending {
				t.Fatal("missing fresh import identity")
			}
			if err := store.BeginEnvironmentCreateWithResources(ctx, lease, plans); err != nil {
				t.Fatal(err)
			}
			if mode == "wrong-mode" {
				if _, err := manager.MaterializeEnvironmentResources(ctx, lease); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("ordinary create accepted imported bytes", err)
				}
				if backend.copies != 0 {
					t.Fatal("provider called on wrong mode")
				}
			}
			areas, err := manager.ImportEnvironmentResources(ctx, lease, inputs)
			if mode == "unknown" || mode == "verify" {
				if err == nil {
					t.Fatal("incomplete import published")
				}
			} else if err != nil || len(areas) != 1 || areas[0].Resource.ImportPending {
				t.Fatal("import incomplete", areas, err)
			}
			absent, err := store.PrepareEnvironmentResourceDeletion(ctx, lease)
			if err != nil {
				t.Fatal(err)
			}
			err = manager.DeleteEnvironmentResources(ctx, absent)
			if mode == "unknown" || mode == "delete" {
				if err == nil {
					t.Fatal("ambiguous import discarded")
				}
				if err := store.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("lost parent reservation", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := store.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
