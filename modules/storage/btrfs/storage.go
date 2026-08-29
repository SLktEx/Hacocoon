package btrfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

const defaultSafetyMargin int64 = 4 << 30

var storageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Storage struct {
	rootDir string
	block   block.Store
	fs      Filesystem
}

func New(rootDir string, backend block.Store, fs Filesystem) *Storage {
	return &Storage{rootDir: rootDir, block: backend, fs: fs}
}

func (*Storage) ID() string { return "storage.btrfs" }

func (s *Storage) Probe(ctx context.Context) (core.StorageCapabilities, error) {
	blockCaps, err := s.block.Probe(ctx)
	if err != nil {
		return core.StorageCapabilities{}, err
	}
	if !blockCaps.Available {
		return core.StorageCapabilities{Available: false, Backend: s.block.ID(), Details: blockCaps.Details}, nil
	}
	if err := s.fs.Probe(ctx); err != nil {
		return core.StorageCapabilities{Available: false, Backend: s.block.ID(), Details: []string{"btrfs unavailable"}}, nil
	}
	return core.StorageCapabilities{
		Available: true,
		Backend:   s.block.ID(),
		Shrink:    blockCaps.Shrink,
		Compact:   blockCaps.Compact,
	}, nil
}

func (s *Storage) Ensure(ctx context.Context, spec core.StorageSpec) (core.StorageHandle, error) {
	if err := validateStorageID(spec.ID); err != nil {
		return core.StorageHandle{}, err
	}
	var out core.StorageHandle
	err := s.withLease(spec.ID, func() error {
		blockHandle, err := s.block.Ensure(ctx, block.Spec{ID: spec.ID, Path: s.imagePath(spec.ID), SizeBytes: spec.SizeBytes})
		if err != nil {
			return err
		}
		mountpoint := s.mountPath(spec.ID)
		if err := s.fs.Ensure(ctx, blockHandle.Device, mountpoint); err != nil {
			return err
		}
		out = core.StorageHandle{
			ID: spec.ID,
			Attachment: map[string]string{
				"driver":     "btrfs",
				"source":     mountpoint,
				"incus_pool": "haco-" + spec.ID,
			},
		}
		return nil
	})
	return out, err
}

func (s *Storage) Inspect(ctx context.Context, handle core.StorageHandle) (core.StorageState, error) {
	if err := validateStorageID(handle.ID); err != nil {
		return core.StorageState{}, err
	}
	fsState, err := s.fs.Inspect(ctx, s.mountPath(handle.ID))
	if err != nil {
		return core.StorageState{}, err
	}
	return core.StorageState{
		Healthy:      fsState.Healthy,
		Backend:      s.block.ID(),
		LogicalBytes: fsState.LogicalBytes,
		UsedBytes:    fsState.UsedBytes,
	}, nil
}

func (s *Storage) Delete(ctx context.Context, handle core.StorageHandle) error {
	if err := validateStorageID(handle.ID); err != nil {
		return err
	}
	return s.withLease(handle.ID, func() error {
		if err := s.fs.Unmount(ctx, s.mountPath(handle.ID)); err != nil {
			return fmt.Errorf("unmount storage %s before deleting backing image: %w", handle.ID, err)
		}
		return s.block.Delete(ctx, s.blockHandle(handle.ID))
	})
}

func (s *Storage) Grow(ctx context.Context, handle core.StorageHandle, target int64) error {
	if err := validateStorageID(handle.ID); err != nil {
		return err
	}
	return s.withLease(handle.ID, func() error {
		current, err := s.fs.Inspect(ctx, s.mountPath(handle.ID))
		if err != nil {
			return err
		}
		if target <= current.LogicalBytes {
			return fmt.Errorf("grow target %d must exceed current size %d: %w", target, current.LogicalBytes, core.ErrInvalidArgument)
		}
		blockHandle, err := s.block.Attach(ctx, s.blockHandle(handle.ID))
		if err != nil {
			return err
		}
		if _, err := s.block.Grow(ctx, blockHandle, target); err != nil {
			return err
		}
		if err := s.fs.Grow(ctx, s.mountPath(handle.ID)); err != nil {
			return fmt.Errorf("outer grow succeeded but filesystem grow needs retry: %w", err)
		}
		return s.fs.Verify(ctx, s.mountPath(handle.ID), 0)
	})
}

func (s *Storage) PlanShrink(ctx context.Context, handle core.StorageHandle, target int64) (core.ShrinkPlan, error) {
	if err := validateStorageID(handle.ID); err != nil {
		return core.ShrinkPlan{}, err
	}
	var plan core.ShrinkPlan
	err := s.withLease(handle.ID, func() error {
		var err error
		plan, err = s.planShrinkUnlocked(ctx, handle, target)
		return err
	})
	return plan, err
}

func (s *Storage) Shrink(ctx context.Context, handle core.StorageHandle, supplied core.ShrinkPlan) error {
	if err := validateStorageID(handle.ID); err != nil {
		return err
	}
	return s.withLease(handle.ID, func() error {
		plan, err := s.planShrinkUnlocked(ctx, handle, supplied.TargetBytes)
		if err != nil {
			return err
		}
		if !plan.Feasible {
			return fmt.Errorf("%w: %s", core.ErrUnsafeShrink, plan.Reason)
		}
		if plan.RequiresCompaction {
			if err := s.fs.Compact(ctx, s.mountPath(handle.ID)); err != nil {
				return fmt.Errorf("compaction failed; outer image unchanged: %w", err)
			}
			plan, err = s.planShrinkUnlocked(ctx, handle, supplied.TargetBytes)
			if err != nil {
				return err
			}
			if !plan.Feasible {
				return fmt.Errorf("%w after compaction: %s", core.ErrUnsafeShrink, plan.Reason)
			}
		}

		mountpoint := s.mountPath(handle.ID)
		if err := s.fs.Shrink(ctx, mountpoint, plan.TargetBytes); err != nil {
			return fmt.Errorf("filesystem shrink failed; outer image unchanged: %w", err)
		}
		if err := s.fs.Unmount(ctx, mountpoint); err != nil {
			return fmt.Errorf("filesystem shrunk but unmount failed; outer image unchanged: %w", err)
		}

		attached, err := s.block.Attach(ctx, s.blockHandle(handle.ID))
		if err != nil {
			return s.recoverMount(ctx, handle.ID, block.Handle{}, fmt.Errorf("reattach before outer shrink: %w", err))
		}
		resized, err := s.block.Shrink(ctx, attached, plan.TargetBytes)
		if err != nil {
			return s.recoverMount(ctx, handle.ID, attached, fmt.Errorf("outer shrink failed after filesystem shrink: %w", err))
		}
		if err := s.fs.Mount(ctx, resized.Device, mountpoint); err != nil {
			return fmt.Errorf("outer shrink succeeded but remount failed; recovery required: %w", err)
		}
		return s.fs.Verify(ctx, mountpoint, plan.TargetBytes)
	})
}

func (s *Storage) Compact(ctx context.Context, handle core.StorageHandle) error {
	if err := validateStorageID(handle.ID); err != nil {
		return err
	}
	return s.withLease(handle.ID, func() error {
		if err := s.fs.Compact(ctx, s.mountPath(handle.ID)); err != nil {
			return err
		}
		return s.block.Compact(ctx, s.blockHandle(handle.ID))
	})
}

func (s *Storage) planShrinkUnlocked(ctx context.Context, handle core.StorageHandle, target int64) (core.ShrinkPlan, error) {
	state, err := s.fs.Inspect(ctx, s.mountPath(handle.ID))
	if err != nil {
		return core.ShrinkPlan{}, err
	}
	minimum, minErr := s.fs.MinimumSize(ctx, s.mountPath(handle.ID))
	if minErr != nil {
		minimum = state.UsedBytes
	}
	margin := defaultSafetyMargin
	if tenPercent := state.LogicalBytes / 10; tenPercent > margin {
		margin = tenPercent
	}
	safeMinimum := minimum + margin
	plan := core.ShrinkPlan{
		HandleID:          handle.ID,
		CurrentBytes:      state.LogicalBytes,
		TargetBytes:       target,
		MinimumBytes:      safeMinimum,
		SafetyMarginBytes: margin,
		Feasible:          target >= safeMinimum && target < state.LogicalBytes,
	}
	plan.RequiresCompaction = plan.Feasible && target < state.LogicalBytes-(state.LogicalBytes/5)
	switch {
	case target >= state.LogicalBytes:
		plan.Reason = "target is not smaller than current logical size"
	case target < safeMinimum:
		plan.Reason = fmt.Sprintf("target below safe minimum; minimum currently %d bytes", safeMinimum)
	default:
		plan.Reason = "target passes current preflight"
	}
	return plan, nil
}

func (s *Storage) recoverMount(ctx context.Context, id string, candidate block.Handle, cause error) error {
	handle := candidate
	if handle.Device == "" {
		attached, err := s.block.Attach(ctx, s.blockHandle(id))
		if err != nil {
			return errors.Join(cause, fmt.Errorf("automatic reattach failed: %w", err))
		}
		handle = attached
	}
	if err := s.fs.Mount(ctx, handle.Device, s.mountPath(id)); err != nil {
		return errors.Join(cause, fmt.Errorf("automatic remount failed: %w", err))
	}
	return cause
}

func (s *Storage) blockHandle(id string) block.Handle {
	return block.Handle{ID: id, Path: s.imagePath(id)}
}

func (s *Storage) imagePath(id string) string {
	ext := ".raw"
	if s.block.ID() == "block.local-qcow2" {
		ext = ".qcow2"
	}
	return filepath.Join(s.rootDir, "images", id+ext)
}

func (s *Storage) mountPath(id string) string {
	return filepath.Join(s.rootDir, "mounts", id)
}

func (s *Storage) withLease(id string, fn func() error) error {
	if err := validateStorageID(id); err != nil {
		return err
	}
	locks := filepath.Join(s.rootDir, "locks")
	if err := os.MkdirAll(locks, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(locks, id+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return fn()
}

func validateStorageID(id string) error {
	if !storageIDPattern.MatchString(id) {
		return fmt.Errorf("storage id %q: %w", id, core.ErrInvalidArgument)
	}
	return nil
}
