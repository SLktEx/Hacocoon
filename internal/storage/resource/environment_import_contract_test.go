package persistentresource_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportPlanRejectsMalformedInventoryBeforeCatalogMutation(t *testing.T) {
	for _, mode := range []string{"empty", "too-many", "missing-archive", "invalid-key", "invalid-digest", "invalid-kind", "unordered", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			_, base, request, _ := environmentContractService(t)
			backend := &importedDataBackend{&savedDataBackend{t: t, store: base.store}}
			manager := &persistentresource.Service{Store: base.store, Backend: backend}
			inputs := []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: strings.Repeat("a", 64), Archive: strings.NewReader("saved bytes")}}
			switch mode {
			case "empty":
				inputs = nil
			case "too-many":
				inputs = make([]core.EnvironmentResourceImport, core.MaxEnvironmentAttachments+1)
			case "missing-archive":
				inputs[0].Archive = nil
			case "invalid-key":
				inputs[0].Key = "../compiler"
			case "invalid-digest":
				inputs[0].Digest = "unverified"
			case "invalid-kind":
				inputs[0].Kind = ""
			case "unordered", "duplicate":
				inputs = append(inputs, inputs[0])
				if mode == "unordered" {
					inputs[1].Key = "before-compiler"
				}
			}
			before, err := os.ReadFile(base.path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.PlanImportedEnvironmentResources(context.Background(), request, inputs); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal("malformed imported inventory accepted", err)
			}
			after, err := os.ReadFile(base.path)
			if err != nil || !bytes.Equal(before, after) || backend.copies != 0 {
				t.Fatal("malformed inventory reserved generations or imported data", err)
			}
		})
	}
}

func TestImportRejectsChangedInputBeforeMaterializingReservedData(t *testing.T) {
	for _, mode := range []string{"empty", "extra", "archive", "key", "target", "kind", "digest", "selected-origin"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			_, base, request, _ := environmentContractService(t)
			backend := &importedDataBackend{&savedDataBackend{t: t, store: base.store}}
			manager := &persistentresource.Service{Store: base.store, Backend: backend}
			inputs := []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: strings.Repeat("a", 64), Archive: strings.NewReader("saved bytes")}}
			plans, err := manager.PlanImportedEnvironmentResources(ctx, request, inputs)
			if err != nil {
				t.Fatal(err)
			}
			lease := core.WorkspaceLease{EnvironmentID: request.EnvironmentID, Owner: request.EnvironmentID, InstanceID: request.InstanceID, WorkspaceID: request.Workspace.ID, SourcePath: request.Workspace.Path, AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), Attachments: []core.EnvironmentAttachment{plans[0].Attachment}}
			if err := base.store.BeginEnvironmentCreateWithResources(ctx, lease, plans); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(base.path)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "empty":
				inputs = nil
			case "extra":
				inputs = append(inputs, inputs[0])
			case "archive":
				inputs[0].Archive = nil
			case "key":
				inputs[0].Key = "packages"
			case "target":
				inputs[0].Target += "/different"
			case "kind":
				inputs[0].Kind = "oci-containerd"
			case "digest":
				inputs[0].Digest = strings.Repeat("b", 64)
			case "selected-origin":
				lease.Attachments[0].Origin.Current = core.PersistentResourceRef{ID: "generation:" + strings.Repeat("c", 32), Owner: strings.Repeat("d", 32)}
			}
			if _, err := manager.ImportEnvironmentResources(ctx, lease, inputs); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal("changed import was accepted", err)
			}
			after, err := os.ReadFile(base.path)
			if err != nil || !bytes.Equal(before, after) || backend.copies != 0 {
				t.Fatal("changed import altered the reservation", err)
			}
			// The original reserved import remains usable after the refusal.
			lease.Attachments[0] = plans[0].Attachment
			inputs = []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: plans[0].Attachment.Origin.Compatibility, Archive: strings.NewReader("saved bytes")}}
			if areas, err := manager.ImportEnvironmentResources(ctx, lease, inputs); err != nil || len(areas) != 1 || areas[0].Resource.Ref() != plans[0].Resource.Ref() || backend.copies != 1 {
				t.Fatal("refusal damaged the valid import reservation", areas, err)
			}
		})
	}
}

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
			store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
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
