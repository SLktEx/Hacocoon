package localqcow2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type Store struct {
	runner       host.Runner
	devRoot      string
	sysBlockRoot string
	procRoot     string
	nbdLockPath  string
}

func New(runner host.Runner) *Store {
	return &Store{runner: runner, devRoot: "/dev", sysBlockRoot: "/sys/block", procRoot: "/proc", nbdLockPath: defaultNBDAllocatorLockPath}
}

func (*Store) ID() string { return "block.local-qcow2" }

func (s *Store) Probe(ctx context.Context) (block.Capabilities, error) {
	for _, command := range []string{"qemu-img", "qemu-nbd"} {
		if _, err := s.runner.Run(ctx, command, "--version"); err != nil {
			return block.Capabilities{Available: false, Details: []string{command + " unavailable"}}, nil
		}
	}
	devices, err := filepath.Glob(filepath.Join(s.devRootPath(), "nbd*"))
	if err != nil {
		return block.Capabilities{}, err
	}
	if len(devices) == 0 {
		return block.Capabilities{Available: false, Details: []string{"no NBD devices; load nbd support first"}}, nil
	}
	return block.Capabilities{Available: true, Shrink: true, Compact: true}, nil
}

func (s *Store) Ensure(ctx context.Context, spec block.Spec) (block.Handle, error) {
	if err := block.PrepareBackingDirectory(spec.Path); err != nil {
		return block.Handle{}, fmt.Errorf("prepare qcow2 backing directory: %w", err)
	}
	info, err := block.ValidateBackingPath(spec.Path, true)
	if err != nil {
		return block.Handle{}, err
	}
	if info == nil {
		if _, err := s.runner.Run(ctx, "qemu-img", "create", "-f", "qcow2", spec.Path, strconv.FormatInt(spec.SizeBytes, 10)); err != nil {
			return block.Handle{}, fmt.Errorf("create qcow2 image: %w", err)
		}
		if _, err := block.ValidateBackingPath(spec.Path, false); err != nil {
			return block.Handle{}, fmt.Errorf("validate created qcow2 image: %w", err)
		}
	}
	return s.Attach(ctx, block.Handle{ID: spec.ID, Path: spec.Path, Bytes: spec.SizeBytes})
}

func (s *Store) Inspect(ctx context.Context, handle block.Handle) (block.State, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.State{}, err
	}
	device := ""
	if err := s.withNBDAllocatorLock(handle.Path, func() error {
		resolved, attached, err := s.resolveNBDLocked(handle)
		if err != nil {
			return err
		}
		if attached {
			device = resolved
		}
		return nil
	}); err != nil {
		return block.State{}, err
	}
	result, err := s.runner.Run(ctx, "qemu-img", "info", "--output=json", handle.Path)
	if err != nil {
		return block.State{}, err
	}
	return block.State{Healthy: true, Bytes: handle.Bytes, Device: device, Details: []string{strings.TrimSpace(result.Stdout)}}, nil
}

func (s *Store) Attach(ctx context.Context, handle block.Handle) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	var out block.Handle
	err := s.withNBDAllocatorLock(handle.Path, func() error {
		var err error
		out, err = s.attachLocked(ctx, handle)
		return err
	})
	return out, err
}

func (s *Store) attachLocked(ctx context.Context, handle block.Handle) (block.Handle, error) {
	device, attached, err := s.resolveNBDLocked(handle)
	if err != nil {
		return block.Handle{}, err
	}
	if attached {
		handle.Device = device
		return handle, nil
	}
	device, err = s.findFreeNBDLocked()
	if err != nil {
		return block.Handle{}, err
	}
	_, connectErr := s.runner.Run(ctx, "qemu-nbd", "--connect="+device, handle.Path)
	verifyErr := s.waitForNBDMatch(ctx, device, handle.Path)
	if connectErr != nil && verifyErr != nil {
		if inspection := s.inspectNBD(device, handle.Path); inspection.observation == nbdFree {
			return block.Handle{}, fmt.Errorf("attach qcow2 via nbd: %w", connectErr)
		}
		return block.Handle{}, errors.Join(fmt.Errorf("attach qcow2 via nbd: %w", connectErr), verifyErr, core.ErrRecoveryRequired)
	}
	if verifyErr != nil {
		return block.Handle{}, verifyErr
	}
	if err := s.writeNBDIdentity(handle.Path, device); err != nil {
		cleanupErr := s.disconnectVerifiedLocked(ctx, device, handle.Path)
		if cleanupErr != nil {
			return block.Handle{}, errors.Join(fmt.Errorf("persist NBD identity after attach: %w", err), cleanupErr, core.ErrRecoveryRequired)
		}
		return block.Handle{}, fmt.Errorf("persist NBD identity after attach: %w", err)
	}
	handle.Device = device
	return handle, nil
}

func (s *Store) Detach(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return err
	}
	return s.withNBDAllocatorLock(handle.Path, func() error {
		return s.detachLocked(ctx, handle)
	})
}

func (s *Store) detachLocked(ctx context.Context, handle block.Handle) error {
	device, attached, err := s.resolveNBDLocked(handle)
	if err != nil {
		return err
	}
	if !attached {
		return removeNBDIdentity(handle.Path)
	}
	if err := s.disconnectVerifiedLocked(ctx, device, handle.Path); err != nil {
		return err
	}
	return removeNBDIdentity(handle.Path)
}

func (s *Store) disconnectVerifiedLocked(ctx context.Context, device, backing string) error {
	inspection := s.inspectNBD(device, backing)
	if inspection.observation != nbdMatches {
		return errors.Join(fmt.Errorf("refusing to disconnect unverified NBD device %s for %s: %s", device, backing, inspection.reason), core.ErrRecoveryRequired)
	}
	if _, err := s.runner.Run(ctx, "qemu-nbd", "--disconnect", device); err != nil {
		return fmt.Errorf("disconnect verified NBD device %s: %w", device, err)
	}
	return s.waitForNBDFree(ctx, device)
}

func (s *Store) Grow(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	var out block.Handle
	err := s.withNBDAllocatorLock(handle.Path, func() error {
		if err := s.detachLocked(ctx, handle); err != nil {
			return err
		}
		if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
			return err
		}
		if _, err := s.runner.Run(ctx, "qemu-img", "resize", handle.Path, strconv.FormatInt(target, 10)); err != nil {
			return fmt.Errorf("grow qcow2: %w", err)
		}
		handle.Bytes = target
		handle.Device = ""
		var err error
		out, err = s.attachLocked(ctx, handle)
		return err
	})
	return out, err
}

func (s *Store) Shrink(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	var out block.Handle
	err := s.withNBDAllocatorLock(handle.Path, func() error {
		if err := s.detachLocked(ctx, handle); err != nil {
			return fmt.Errorf("detach before qcow2 shrink: %w", err)
		}
		if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
			return err
		}
		if _, err := s.runner.Run(ctx, "qemu-img", "resize", "--shrink", handle.Path, strconv.FormatInt(target, 10)); err != nil {
			return fmt.Errorf("shrink qcow2: %w", err)
		}
		if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
			return err
		}
		if _, err := s.runner.Run(ctx, "qemu-img", "check", handle.Path); err != nil {
			return fmt.Errorf("verify qcow2 after shrink: %w", err)
		}
		handle.Bytes = target
		handle.Device = ""
		var err error
		out, err = s.attachLocked(ctx, handle)
		return err
	})
	return out, err
}

func (s *Store) Compact(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return err
	}
	return s.withNBDAllocatorLock(handle.Path, func() error {
		device, attached, err := s.resolveNBDLocked(handle)
		if err != nil {
			return err
		}
		if attached {
			return fmt.Errorf("qcow2 compact requires detached image; verified live device is %s", device)
		}
		_, err = s.runner.Run(ctx, "qemu-img", "check", handle.Path)
		return err
	})
}

func (s *Store) Delete(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return removeNBDIdentity(handle.Path)
		}
		return err
	}
	return s.withNBDAllocatorLock(handle.Path, func() error {
		if err := s.detachLocked(ctx, handle); err != nil {
			return fmt.Errorf("detach qcow2 NBD image before delete: %w", err)
		}
		// Reconcile once more immediately before unlink. This catches another
		// Hacocoon process trying to reattach the same backing image after stale
		// state recovery; the global allocator lock keeps those operations out.
		device, attached, err := s.resolveNBDLocked(block.Handle{ID: handle.ID, Path: handle.Path, Bytes: handle.Bytes})
		if err != nil {
			return err
		}
		if attached {
			return errors.Join(fmt.Errorf("refusing to delete qcow2 still attached at %s", device), core.ErrRecoveryRequired)
		}
		if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
			return err
		}
		if err := os.Remove(handle.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return removeNBDIdentity(handle.Path)
	})
}
