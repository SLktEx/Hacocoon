package workspace

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// LegacyRestoreBackend only cleans up ownership receipts from retired staging.
// New Snapshot consumers always create a new Environment.
type LegacyRestoreBackend interface {
	DeleteRestoreComponent(context.Context, core.SnapshotComponent) error
}
type restoreCatalog interface {
	GetSnapshotRestore(context.Context, string) (core.SnapshotRestore, error)
	RecordRestoreComponent(context.Context, string, core.SnapshotComponent, string) error
	BeginRestoreCleanup(context.Context, string) error
	FinalizeRestoreCleanup(context.Context, string) error
}

// CleanupLegacySnapshotRestore discards only the staged replacement. The current
// Environment and saved snapshots remain owned and unchanged.
func (s *Service) CleanupLegacySnapshotRestore(ctx context.Context, id string) error {
	backend, ok := s.runtime.(LegacyRestoreBackend)
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
	unlock, err := s.lockLifecycle(ctx, "environment", op.Current.Environment.Name)
	if err != nil {
		return err
	}
	defer unlock()
	release, err := s.lockWorkspace(ctx, op.Current.Environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	return cleanupSnapshotRestoreLocked(ctx, catalog, backend, op)
}

// Caller holds the canonical Environment and Workspace locks. Both explicit
// cleanup and failure cleanup use the same ownership/positive-absence contract.
func cleanupSnapshotRestoreLocked(ctx context.Context, catalog restoreCatalog, backend LegacyRestoreBackend, op core.SnapshotRestore) error {
	id := op.ID
	current, err := catalog.GetSnapshotRestore(ctx, id)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current.Current, op.Current) || !reflect.DeepEqual(current.Saved, op.Saved) {
		return core.ErrCapabilityStale
	}
	if err := catalog.BeginRestoreCleanup(ctx, id); err != nil {
		return err
	}
	var cleanupErrors []error
	for _, c := range current.Components {
		if c.State == "absent" {
			continue
		}
		if err := backend.DeleteRestoreComponent(ctx, c); err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		if err := catalog.RecordRestoreComponent(ctx, id, c, "absent"); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	if len(cleanupErrors) != 0 {
		return fmt.Errorf("restore %s cleanup incomplete: %w", id, errors.Join(append([]error{core.ErrRecoveryRequired}, cleanupErrors...)...))
	}
	return catalog.FinalizeRestoreCleanup(ctx, id)
}
