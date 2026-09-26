package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

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
func (s *Service) CaptureSnapshot(ctx context.Context, name string) (core.Snapshot, error) {
	return s.captureSnapshot(ctx, name, true, "", "")
}

// CaptureStoppedSnapshot refuses a running source under the canonical locks.
// Copy callers must not stop or restart somebody else's running workload.
func (s *Service) CaptureStoppedSnapshot(ctx context.Context, name string) (core.Snapshot, error) {
	return s.captureSnapshot(ctx, name, false, "", "")
}

// CaptureStoppedSnapshotForWorkspace refuses a recycled Env name under the
// canonical locks before capturing any provider component.
func (s *Service) CaptureStoppedSnapshotForWorkspace(ctx context.Context, name string, expected core.WorkspaceID) (core.Snapshot, error) {
	if expected == "" {
		return core.Snapshot{}, core.ErrInvalidArgument
	}
	return s.captureSnapshot(ctx, name, false, expected, "")
}
func (s *Service) captureSnapshot(ctx context.Context, name string, allowRunning bool, expected core.WorkspaceID, id string) (result core.Snapshot, err error) {
	backend, ok := s.runtime.(SnapshotBackend)
	if !ok {
		return result, core.ErrUnsupported
	}
	catalog, ok := s.store.(snapshotCatalog)
	if !ok {
		return result, core.ErrUnsupported
	}
	if id == "" {
		var nonce [16]byte
		_, _ = rand.Read(nonce[:])
		id = "snap-" + hex.EncodeToString(nonce[:])
	}
	unlock, err := s.LockResourceName(ctx, id)
	if err != nil {
		return result, err
	}
	defer unlock()
	if s.checkSnapshotName != nil {
		if err := s.checkSnapshotName(ctx, id); err != nil {
			return result, err
		}
	}
	err = s.withSnapshotSourceMode(ctx, name, allowRunning, func(ctx context.Context, source core.SnapshotSource) error {
		if expected != "" && source.Environment.Workspace.ID != expected {
			return core.ErrCapabilityStale
		}
		result, err = s.captureSnapshotLocked(ctx, source, backend, catalog, id)
		return err
	})
	return result, err
}

// The caller holds the canonical Environment and Workspace lifecycle locks.
func (s *Service) captureSnapshotLocked(ctx context.Context, source core.SnapshotSource, backend SnapshotBackend, catalog snapshotCatalog, id string) (result core.Snapshot, err error) {
	err = func() error {
		components, err := backend.PlanSnapshot(ctx, source, id)
		if err != nil {
			return err
		}
		planned := core.Snapshot{CreatedAt: time.Now().UTC(), ID: id, Source: source, State: "capturing", Components: components}
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
			if component.Role == "image" {
				reader, ok := s.runtime.(interface {
					SnapshotImage(context.Context, core.SnapshotComponent) (core.BaseRef, error)
				})
				writer, writable := s.store.(interface {
					RecordSnapshotImage(context.Context, string, core.BaseRef) error
				})
				if !ok || !writable {
					return fail(core.ErrUnsupported)
				}
				image, err := reader.SnapshotImage(ctx, component)
				if err != nil {
					return fail(err)
				}
				if err := writer.RecordSnapshotImage(ctx, id, image); err != nil {
					return fail(err)
				}
				result.Image = &image
			}
		}
		if err := catalog.CommitSnapshot(ctx, id); err != nil {
			return fail(err)
		}
		result.State = "ready"
		return nil
	}()
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
	return s.withSavedSnapshot(ctx, id, func(ctx context.Context, current core.Snapshot) error {
		if err := catalog.BeginSnapshotDelete(ctx, id); err != nil {
			return err
		}
		for _, component := range current.Components {
			if component.State == "absent" {
				continue
			}
			if err := backend.DeleteSnapshotComponent(ctx, component); err != nil {
				return fmt.Errorf("snapshot %s component %q cleanup incomplete: %w", id, component.Role, errors.Join(core.ErrRecoveryRequired, err))
			}
			if err := catalog.RecordSnapshotComponent(ctx, id, component, "absent"); err != nil {
				return err
			}
		}
		return catalog.FinalizeSnapshotDelete(ctx, id)
	})
}

func (s *Service) CaptureNamedSnapshot(ctx context.Context, environment, name string) (core.Snapshot, error) {
	if name == "" {
		return core.Snapshot{}, core.ErrInvalidArgument
	}
	return s.captureSnapshot(ctx, environment, true, "", name)
}
