package persistentresource_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	persistentresource "github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type deletionFinalizeFault struct {
	*state.EnvironmentJSONStore
	afterWrite bool
	err        error
}

func (s *deletionFinalizeFault) FinalizePersistentResourceDelete(ctx context.Context, r core.PersistentResource) error {
	if s.afterWrite {
		if err := s.EnvironmentJSONStore.FinalizePersistentResourceDelete(ctx, r); err != nil {
			return err
		}
	}
	return s.err
}

func TestWorkspaceDeletionPreservesOwnershipUntilPositiveAbsence(t *testing.T) {
	for _, mode := range []string{"complete", "foreign-workspace", "empty-workspace", "attached", "native-uncertain", "finalize-before-write", "finalize-after-write"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, b, _, _ := environmentContractService(t)
			source, err := svc.Create(ctx, "oci:source", "oci-containerd")
			if err != nil {
				t.Fatal(err)
			}
			target, err := svc.CopyForWorkspace(ctx, "oci:target", source.Kind, source.ID, "work")
			if err != nil {
				t.Fatal(err)
			}
			work := core.WorkspaceID("work")
			var want error
			switch mode {
			case "foreign-workspace":
				work, want = "other", core.ErrIncompatibleState
			case "empty-workspace":
				work, want = "", core.ErrInvalidArgument
			case "attached":
				lease := core.WorkspaceLease{EnvironmentID: "dev", Owner: "dev", WorkspaceID: "work", SourcePath: "/work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: target.Ref()}
				if err := b.store.BeginEnvironmentCreate(ctx, lease); err != nil {
					t.Fatal(err)
				}
				want = core.ErrStorageBusy
			case "native-uncertain":
				b.failDelete, want = true, core.ErrRecoveryRequired
			case "finalize-before-write", "finalize-after-write":
				svc.Store = &deletionFinalizeFault{EnvironmentJSONStore: b.store, afterWrite: mode == "finalize-after-write", err: errors.New("catalog write reply lost")}
				want = core.ErrRecoveryRequired
			}
			err = svc.DeleteForWorkspace(ctx, target.ID, work)
			if !errors.Is(err, want) {
				t.Fatal("unexpected deletion result", err, want)
			}
			reopened := state.NewEnvironmentJSONStore(b.path)
			held, readErr := reopened.GetPersistentResource(ctx, target.ID)
			switch mode {
			case "foreign-workspace", "empty-workspace", "attached":
				if readErr != nil || held != target || b.deletes != 0 || b.resources[target.ID].Ref() != target.Ref() {
					t.Fatal("refusal changed or deleted owned data", held, readErr, b.deletes)
				}
			case "native-uncertain", "finalize-before-write":
				if readErr != nil || held.Ref() != target.Ref() || held.WorkspaceID != "work" || held.State != "deleting" {
					t.Fatal("uncertain deletion lost its reservation", held, readErr)
				}
				_, exists := b.resources[target.ID]
				if exists != (mode == "native-uncertain") {
					t.Fatal("native absence did not match the failure boundary", exists)
				}
				b.failDelete = false
				svc.Store = reopened
				if err := svc.DeleteForWorkspace(ctx, target.ID, "work"); err != nil {
					t.Fatal("exact-owner retry failed", err)
				}
				if _, err := reopened.GetPersistentResource(ctx, target.ID); !errors.Is(err, core.ErrNotFound) {
					t.Fatal("retry left the deletion reservation", err)
				}
			case "complete", "finalize-after-write":
				if !errors.Is(readErr, core.ErrNotFound) {
					t.Fatal("positive absence was not finalized", held, readErr)
				}
				if _, exists := b.resources[target.ID]; exists {
					t.Fatal("catalog forgotten before native absence")
				}
			}
			if heldSource, err := reopened.GetPersistentResource(ctx, source.ID); err != nil || heldSource != source || b.resources[source.ID].Ref() != source.Ref() {
				t.Fatal("deleting a workspace copy changed the source", heldSource, err)
			}
		})
	}
}

func TestWorkspaceDeletionRequiresAtomicWorkspaceOwnershipContract(t *testing.T) {
	svc, b, _, _ := environmentContractService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "oci:source", "oci-containerd")
	if err != nil {
		t.Fatal(err)
	}
	// An adapter implementing only the basic catalog cannot safely emulate the
	// atomic workspace check by reading metadata and then calling ordinary Delete.
	svc.Store = struct{ persistentresource.Store }{b.store}
	if err := svc.DeleteForWorkspace(ctx, r.ID, "work"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if held, err := b.store.GetPersistentResource(ctx, r.ID); err != nil || held != r || b.deletes != 0 || b.resources[r.ID].Ref() != r.Ref() {
		t.Fatal("unsupported deletion changed owned data", held, err)
	}
}
