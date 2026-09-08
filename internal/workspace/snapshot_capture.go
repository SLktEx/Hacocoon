package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SnapshotBackend is optional. Plan must enumerate the complete owned aggregate
// without mutation. Saved components must remain independent of source deletion.
// Create must use the planned identity; Verify must establish exact ownership and
// completed data. Delete succeeds only after positively establishing absence of
// the exact owned target, including a planned target without a create receipt.
type SnapshotBackend interface {
	PlanSnapshot(context.Context, core.SnapshotSource, string) ([]core.SnapshotComponent, error)
	CreateSnapshotComponent(context.Context, core.SnapshotSource, core.SnapshotComponent) error
	VerifySnapshotComponent(context.Context, core.SnapshotComponent) error
	DeleteSnapshotComponent(context.Context, core.SnapshotComponent) error
}

type snapshotCatalog interface {
	BeginSnapshot(context.Context, core.Snapshot) error
	GetSnapshot(context.Context, string) (core.Snapshot, error)
	RecordSnapshotComponent(context.Context, string, core.SnapshotComponent, string) error
	CommitSnapshot(context.Context, string) error
	MarkSnapshotRecovery(context.Context, string) error
	BeginSnapshotDelete(context.Context, string) error
	FinalizeSnapshotDelete(context.Context, string) error
}

// CaptureSnapshot holds canonical source locks through all capture and publication
// steps. A nonempty result ID on error names a durable reservation for recovery.
func (s *Service) CaptureSnapshot(ctx context.Context, name string) (result core.Snapshot, err error) {
	backend, ok := s.runtime.(SnapshotBackend)
	if !ok {
		return result, core.ErrUnsupported
	}
	catalog, ok := s.store.(snapshotCatalog)
	if !ok {
		return result, core.ErrUnsupported
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return result, err
	}
	id := "snap-" + hex.EncodeToString(nonce[:])
	err = s.withSnapshotSource(ctx, name, func(ctx context.Context, source core.SnapshotSource) error {
		components, err := backend.PlanSnapshot(ctx, source, id)
		if err != nil {
			return err
		}
		planned := core.Snapshot{ID: id, Source: source, State: "capturing", Components: components}
		if err := catalog.BeginSnapshot(ctx, planned); err != nil {
			return err
		}
		result = planned
		fail := func(cause error) error {
			// Cancellation must not erase ownership or prevent the recovery marker.
			recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
			defer cancel()
			markErr := catalog.MarkSnapshotRecovery(recovery, id)
			if markErr == nil {
				result.State = "recovery-required"
			}
			return fmt.Errorf("snapshot %s retained for recovery: %w", id, errors.Join(core.ErrRecoveryRequired, cause, markErr))
		}
		for i, component := range result.Components {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			if err := backend.CreateSnapshotComponent(ctx, source, component); err != nil {
				return fail(err)
			}
			// No fallible provider call may intervene between create and this receipt.
			if err := catalog.RecordSnapshotComponent(ctx, id, component, "created"); err != nil {
				return fail(err)
			}
			component.State = "created"
			result.Components[i] = component
			if err := backend.VerifySnapshotComponent(ctx, component); err != nil {
				return fail(err)
			}
			if err := catalog.RecordSnapshotComponent(ctx, id, component, "verified"); err != nil {
				return fail(err)
			}
			component.State = "verified"
			result.Components[i] = component
		}
		if err := catalog.CommitSnapshot(ctx, id); err != nil {
			return fail(err)
		}
		result.State = "ready"
		return nil
	})
	return result, err
}

// DeleteSnapshot also recovers partial captures. It never assumes that a failed
// create means absence, and leaves the catalog intact on ambiguous cleanup.
func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	backend, ok := s.runtime.(SnapshotBackend)
	if !ok {
		return core.ErrUnsupported
	}
	catalog, ok := s.store.(snapshotCatalog)
	if !ok {
		return core.ErrUnsupported
	}
	snapshot, err := catalog.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	unlock, err := lockLifecycle(ctx, "environment", snapshot.Source.Environment.Name)
	if err != nil {
		return err
	}
	defer unlock()
	release, err := lockWorkspace(ctx, snapshot.Source.Environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	current, err := catalog.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	if current.ID != snapshot.ID || !reflect.DeepEqual(current.Source, snapshot.Source) {
		return core.ErrCapabilityStale
	}
	if err := catalog.BeginSnapshotDelete(ctx, id); err != nil {
		return err
	}
	for _, component := range current.Components {
		if component.State == "absent" {
			continue
		}
		if err := backend.DeleteSnapshotComponent(ctx, component); err != nil {
			return fmt.Errorf("snapshot %s cleanup incomplete: %w", id, errors.Join(core.ErrRecoveryRequired, err))
		}
		if err := catalog.RecordSnapshotComponent(ctx, id, component, "absent"); err != nil {
			return err
		}
	}
	return catalog.FinalizeSnapshotDelete(ctx, id)
}
