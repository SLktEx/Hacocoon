package persistentresource_test

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"testing"
)

type trackedGenerationBackend struct {
	*environmentContractBackend
	stopErr   error
	complete  bool
	noReceipt bool
	cancel    context.CancelFunc
	target    core.PersistentResource
}

func (b *trackedGenerationBackend) CopyTracked(ctx context.Context, source, target core.PersistentResource, record func(string) error, completed func() error) error {
	b.target = target
	if err := b.Copy(ctx, source, target); err != nil {
		return err
	}
	if !b.noReceipt {
		if err := record("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"); err != nil {
			return err
		}
	}
	if b.complete {
		return completed()
	}
	if b.cancel != nil {
		b.cancel()
	}
	return errors.New("copy interrupted")
}
func (b *trackedGenerationBackend) WaitCopyStopped(ctx context.Context, target core.PersistentResource) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	saved, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || saved != target || !saved.CopyCleanup || saved.CopyCompleted {
		return errors.New("stop without durable cleanup fence")
	}
	if err := b.store.MarkPersistentResourceCopyCompleted(ctx, target); err == nil {
		return errors.New("cleanup allowed publication")
	}
	if err := b.store.CheckEnvironmentResourcesIdle(ctx, "builder"); !errors.Is(err, core.ErrRecoveryRequired) {
		return errors.New("cleanup released source too early")
	}
	return b.stopErr
}
func TestCacheCopyFailureCleansOnlyStoppedOwnedDestination(t *testing.T) {
	for _, mode := range []string{"failed", "cancelled", "running", "delete-failed", "missing-receipt", "complete"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			svc, base, lease := activeResourceEnvironment(t)
			b := &trackedGenerationBackend{environmentContractBackend: base, complete: mode == "complete", noReceipt: mode == "missing-receipt"}
			if mode == "cancelled" {
				b.cancel = cancel
			}
			if mode == "running" {
				b.stopErr = core.ErrRecoveryRequired
			}
			base.failDelete = mode == "delete-failed"
			svc.Backend = b
			source := lease.Attachments[0].Resource
			result, err := svc.PublishEnvironmentGeneration(ctx, lease, lease.Attachments[0])
			check := context.Background()
			if mode == "complete" {
				if err != nil || result.State != "published" || base.deletes != 0 || result.Candidate.CopyOperation != "" {
					t.Fatal(result, err)
				}
				return
			}
			if err == nil {
				t.Fatal("lost original copy failure")
			}
			if mode == "failed" || mode == "cancelled" {
				if result.State != "cleaned" || result.Candidate != (core.PersistentResource{}) || base.deletes != 1 {
					t.Fatal(result, err, base.deletes)
				}
			} else {
				reopened := state.NewEnvironmentJSONStore(base.path)
				svc.Store = reopened
				held, readErr := reopened.GetPersistentResource(check, b.target.ID)
				if readErr != nil || held.CopySource != source || held.CopyCompleted {
					t.Fatal(held, readErr)
				}
				if err := reopened.CheckEnvironmentResourcesIdle(check, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("source not fenced", err)
				}
				if mode == "missing-receipt" {
					if _, err := svc.RecoverEnvironmentGeneration(check, held.Ref()); !errors.Is(err, core.ErrRecoveryRequired) || base.deletes != 0 {
						t.Fatal("unknown request deleted", err)
					}
					return
				}
				foreign := held.Ref()
				foreign.Owner = "ffffffffffffffffffffffffffffffff"
				if _, err := svc.RecoverEnvironmentGeneration(check, foreign); !errors.Is(err, core.ErrCapabilityStale) {
					t.Fatal(err)
				}
				b.stopErr = nil
				base.failDelete = false
				result, err = svc.RecoverEnvironmentGeneration(check, held.Ref())
				if err != nil || result.State != "cleaned" {
					t.Fatal(result, err)
				}
			}
			if _, err := svc.Store.GetPersistentResource(check, b.target.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("target survived", err)
			}
			if saved, err := svc.Store.GetPersistentResource(check, source.ID); err != nil || saved.Ref() != source || base.resources[source.ID].Ref() != source {
				t.Fatal("source changed", saved, err)
			}
			if err := base.store.CheckEnvironmentResourcesIdle(check, lease.EnvironmentID); err != nil {
				t.Fatal("source remains blocked", err)
			}
			g, err := base.store.GetResourceGeneration(check, lease.Attachments[0].Origin.Name)
			if err != nil || g != lease.Attachments[0].Origin {
				t.Fatal("failed copy changed selection", g, err)
			}
		})
	}
}

// A persistence error after the completion receipt reached disk must not turn
// a complete generation into failed-copy deletion.
type completedReceiptReplyLostStore struct{ *state.EnvironmentJSONStore }

func (s completedReceiptReplyLostStore) MarkPersistentResourceCopyCompleted(ctx context.Context, r core.PersistentResource) error {
	if err := s.EnvironmentJSONStore.MarkPersistentResourceCopyCompleted(ctx, r); err != nil {
		return err
	}
	return errors.New("completion receipt reply lost")
}
func TestTrackedCacheCopyPreservesAmbiguousCompletedReceipt(t *testing.T) {
	ctx := context.Background()
	svc, base, lease := activeResourceEnvironment(t)
	b := &trackedGenerationBackend{environmentContractBackend: base, complete: true}
	svc.Backend = b
	svc.Store = completedReceiptReplyLostStore{base.store}
	result, err := svc.PublishEnvironmentGeneration(ctx, lease, lease.Attachments[0])
	if !errors.Is(err, core.ErrRecoveryRequired) || base.deletes != 0 {
		t.Fatal(result, err)
	}
	saved, err := base.store.GetPersistentResource(ctx, b.target.ID)
	if err != nil || !saved.CopyCompleted || saved.CopyCleanup {
		t.Fatal(saved, err)
	}
	svc.Store = base.store
	result, err = svc.RecoverEnvironmentGeneration(ctx, saved.Ref())
	if err != nil || result.State != "published" || base.deletes != 0 {
		t.Fatal(result, err)
	}
}
