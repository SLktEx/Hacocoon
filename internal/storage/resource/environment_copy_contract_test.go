package persistentresource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type interruptedEnvironmentCopyBackend struct {
	*maintenanceBackend
	missingReceipt bool
	recoveries     int
}

func (b *interruptedEnvironmentCopyBackend) CopyWithCompletion(ctx context.Context, source, target core.PersistentResource, completed func() error) error {
	if err := b.Copy(ctx, source, target); err != nil {
		return err
	}
	if b.missingReceipt {
		// Provider success alone cannot replace its required positive receipt.
		return nil
	}
	if err := completed(); err != nil {
		return err
	}
	return errors.New("native copy finished but source restoration reply was lost")
}

func (b *interruptedEnvironmentCopyBackend) RecoverCompletedCopy(ctx context.Context, source, target core.PersistentResource) error {
	b.recoveries++
	held, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || held != target || !held.CopyCompleted || held.CopySource != source.Ref() {
		return errors.New("source restoration without a durable completed copy")
	}
	return nil
}

func TestEnvironmentMaterializesSelectedSourceAndCleansOnlyItsOwnCopy(t *testing.T) {
	for _, mode := range []string{"complete", "missing-completion", "interrupted-restoration"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, base, request, selections := environmentContractService(t)
			published, err := svc.PublishGeneration(ctx, selections[0].Origin, preparedGeneration)
			if err != nil {
				t.Fatal(err)
			}
			selections[0].Origin = published.Generation
			lease, plans := reserveEnvironmentContract(t, svc, base.store, request, selections[:1])
			b := &interruptedEnvironmentCopyBackend{maintenanceBackend: &maintenanceBackend{environmentContractBackend: base}, missingReceipt: mode == "missing-completion"}
			svc.Backend = b
			if mode == "complete" {
				svc.Backend = b.maintenanceBackend
			}
			areas, err := svc.MaterializeEnvironmentResources(ctx, lease)
			if mode == "complete" {
				if err != nil || len(areas) != 1 || areas[0].Resource.State != "ready" || areas[0].Resource.CopySource != (core.PersistentResourceRef{}) {
					t.Fatal("selected data was not materialized", areas, err)
				}
			} else if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("interrupted copy was published", areas, err)
			}
			reopened := state.NewEnvironmentJSONStore(base.path)
			svc.Store = reopened
			target, err := reopened.GetPersistentResource(ctx, plans[0].Resource.ID)
			if err != nil || target.Ref() != plans[0].Resource.Ref() || target.EnvironmentInstance != lease.InstanceID || b.copies != 1 || len(base.resources) != 2 {
				t.Fatal("copy lost fresh durable ownership", target, err)
			}
			absent, err := reopened.PrepareEnvironmentResourceDeletion(ctx, lease)
			if err != nil {
				t.Fatal(err)
			}
			err = svc.DeleteEnvironmentResources(ctx, absent)
			if mode == "missing-completion" {
				if !errors.Is(err, core.ErrRecoveryRequired) || b.recoveries != 0 || base.deletes != 0 || len(base.resources) != 2 {
					t.Fatal("unknown copy was resumed or deleted", err)
				}
				if err := reopened.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("unfinished child released the parent", err)
				}
			} else {
				wantRecoveries := 0
				if mode == "interrupted-restoration" {
					wantRecoveries = 1
				}
				if err != nil || b.recoveries != wantRecoveries || base.deletes != 1 || len(base.resources) != 1 {
					t.Fatal("owned copy cleanup failed", err, b.recoveries, base.deletes)
				}
				if err := svc.DeleteEnvironmentResources(ctx, absent); err != nil || base.deletes != 1 {
					t.Fatal("repeated cleanup deleted another native resource", err)
				}
				if err := reopened.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); err != nil {
					t.Fatal("positive cleanup retained the parent", err)
				}
			}
			current, err := reopened.GetResourceGeneration(ctx, published.Generation.Name)
			if err != nil || current != published.Generation || base.resources[current.Current.ID].Ref() != current.Current || b.copies != 1 {
				t.Fatal("copy cleanup changed the selected source", current, err)
			}
		})
	}
}
