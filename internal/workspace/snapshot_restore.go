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

// RestoreBackend prepares independently owned storage. It never replaces or
// starts an Environment. Plan is read-only; Create returns before any fallible
// verification so the service can first persist its completion receipt.
type RestoreBackend interface {
	PlanSnapshotRestore(context.Context, core.Snapshot, string) ([]core.SnapshotComponent, error)
	CreateRestoreComponent(context.Context, core.Snapshot, core.SnapshotComponent) error
	VerifyRestoreComponent(context.Context, core.SnapshotComponent) error
	DeleteRestoreComponent(context.Context, core.SnapshotComponent) error
}
type restoreCatalog interface {
	snapshotCatalog
	BeginSnapshotRestore(context.Context, core.SnapshotRestore) error
	GetSnapshotRestore(context.Context, string) (core.SnapshotRestore, error)
	RecordRestoreComponent(context.Context, string, core.SnapshotComponent, string) error
	CommitRestorePreparation(context.Context, string) error
	MarkRestoreRecovery(context.Context, string) error
	BeginRestoreCleanup(context.Context, string) error
	FinalizeRestoreCleanup(context.Context, string) error
}

// PrepareSnapshotRestore saves current stopped work before copying the complete
// saved aggregate. It supports the original Environment name; cross-Environment
// copy is a separate operation. Prepared storage is not a completed restore.
func (s *Service) PrepareSnapshotRestore(ctx context.Context, name, savedID string) (result core.SnapshotRestore, err error) {
	backend, ok := s.runtime.(RestoreBackend)
	if !ok {
		return result, core.ErrUnsupported
	}
	snapshots, ok := s.runtime.(SnapshotBackend)
	if !ok {
		return result, core.ErrUnsupported
	}
	catalog, ok := s.store.(restoreCatalog)
	if !ok {
		return result, core.ErrUnsupported
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return result, err
	}
	id := "restore-" + hex.EncodeToString(nonce[:])
	err = s.withSnapshotSource(ctx, name, func(ctx context.Context, current core.SnapshotSource) error {
		saved, err := catalog.GetSnapshot(ctx, savedID)
		if err != nil {
			return err
		}
		if saved.State != "ready" || saved.Source.Environment.Name != name {
			return core.ErrIncompatibleState
		}
		for _, c := range saved.Components {
			if c.Binding == "" {
				return core.ErrUnsupported
			}
			if err := snapshots.VerifySnapshotComponent(ctx, c); err != nil {
				return err
			}
		}
		components, err := backend.PlanSnapshotRestore(ctx, saved, id)
		if err != nil {
			return err
		}
		before, err := s.captureSnapshotLocked(ctx, current, snapshots, catalog)
		if err != nil {
			return err
		}
		// Even if reservation fails, the newly captured backup remains discoverable.
		result = core.SnapshotRestore{ID: id, Saved: saved, Before: before, State: "preparing", Components: components}
		if err := catalog.BeginSnapshotRestore(ctx, result); err != nil {
			result.ID = ""
			return fmt.Errorf("pre-restore snapshot %s preserved: %w", before.ID, err)
		}
		fail := func(cause error) error {
			recovery, cancel := s.newCleanupContext(context.WithoutCancel(ctx))
			defer cancel()
			markErr := catalog.MarkRestoreRecovery(recovery, id)
			if markErr == nil {
				result.State = "recovery-required"
			}
			return fmt.Errorf("restore %s retained for recovery: %w", id, errors.Join(core.ErrRecoveryRequired, cause, markErr))
		}
		for i, c := range result.Components {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			if err := backend.CreateRestoreComponent(ctx, saved, c); err != nil {
				return fail(err)
			}
			if err := catalog.RecordRestoreComponent(ctx, id, c, "created"); err != nil {
				return fail(err)
			}
			c.State = "created"
			result.Components[i] = c
			if err := backend.VerifyRestoreComponent(ctx, c); err != nil {
				return fail(err)
			}
			if err := catalog.RecordRestoreComponent(ctx, id, c, "verified"); err != nil {
				return fail(err)
			}
			c.State = "verified"
			result.Components[i] = c
		}
		if err := catalog.CommitRestorePreparation(ctx, id); err != nil {
			return fail(err)
		}
		result.State = "prepared"
		return nil
	})
	return result, err
}

// CleanupSnapshotRestore discards only the staged replacement. The current
// Environment, selected save and pre-restore backup remain owned and unchanged.
func (s *Service) CleanupSnapshotRestore(ctx context.Context, id string) error {
	backend, ok := s.runtime.(RestoreBackend)
	if !ok {
		return core.ErrUnsupported
	}
	catalog, ok := s.store.(restoreCatalog)
	if !ok {
		return core.ErrUnsupported
	}
	op, err := catalog.GetSnapshotRestore(ctx, id)
	if err != nil {
		return err
	}
	unlock, err := lockLifecycle(ctx, "environment", op.Before.Source.Environment.Name)
	if err != nil {
		return err
	}
	defer unlock()
	release, err := lockWorkspace(ctx, op.Before.Source.Environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	current, err := catalog.GetSnapshotRestore(ctx, id)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current.Before, op.Before) || !reflect.DeepEqual(current.Saved, op.Saved) {
		return core.ErrCapabilityStale
	}
	if err := catalog.BeginRestoreCleanup(ctx, id); err != nil {
		return err
	}
	for _, c := range current.Components {
		if c.State == "absent" {
			continue
		}
		if err := backend.DeleteRestoreComponent(ctx, c); err != nil {
			return fmt.Errorf("restore %s cleanup incomplete: %w", id, errors.Join(core.ErrRecoveryRequired, err))
		}
		if err := catalog.RecordRestoreComponent(ctx, id, c, "absent"); err != nil {
			return err
		}
	}
	return catalog.FinalizeRestoreCleanup(ctx, id)
}
