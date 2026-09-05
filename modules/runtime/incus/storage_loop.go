package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// BtrfsLoopPoolSpec describes an Incus-owned loop-backed Btrfs pool.
// Source is intentionally absent: omitting source delegates sparse image,
// loop-device, filesystem and mount lifecycle to Incus itself.
type BtrfsLoopPoolSpec struct {
	Name         string
	Size         string
	MountOptions string
}

// EnsureBtrfsLoopPool ensures an Incus-owned loop-backed Btrfs pool exists.
// It deliberately does not accept a Host source path.
func (r *Runtime) EnsureBtrfsLoopPool(ctx context.Context, spec BtrfsLoopPoolSpec) (string, error) {
	name := strings.TrimSpace(spec.Name)
	size := strings.TrimSpace(spec.Size)
	mountOptions := strings.TrimSpace(spec.MountOptions)
	if name == "" || size == "" {
		return "", fmt.Errorf("Incus Btrfs loop pool requires name and size: %w", core.ErrInvalidArgument)
	}

	if _, err := r.runner.Run(ctx, "incus", "storage", "show", name, "--project", r.project); err == nil {
		if err := r.reconcileBtrfsLoopPoolMountOptions(ctx, name, mountOptions); err != nil {
			return "", err
		}
		return name, nil
	}

	args := []string{"storage", "create", name, "btrfs", "size=" + size}
	if mountOptions != "" {
		args = append(args, "btrfs.mount_options="+mountOptions)
	}
	args = append(args, "--project", r.project)
	result, err := r.runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return "", fmt.Errorf("create Incus-managed Btrfs loop pool %q: %s: %w", name, reason, err)
		}
		return "", fmt.Errorf("create Incus-managed Btrfs loop pool %q: %w", name, err)
	}

	if _, err := r.runner.Run(ctx, "incus", "storage", "show", name, "--project", r.project); err != nil {
		return "", fmt.Errorf("verify Incus-managed Btrfs loop pool %q: %w", name, err)
	}
	return name, nil
}

func (r *Runtime) reconcileBtrfsLoopPoolMountOptions(ctx context.Context, name, desired string) error {
	if desired == "" {
		return nil
	}

	current, err := r.runner.Run(ctx, "incus", "storage", "get", name, "btrfs.mount_options", "--project", r.project)
	if err != nil {
		return fmt.Errorf("inspect Incus-managed Btrfs loop pool %q mount options: %w", name, err)
	}
	if strings.TrimSpace(current.Stdout) == desired {
		return nil
	}

	result, err := r.runner.Run(ctx, "incus", "storage", "set", name, "btrfs.mount_options="+desired, "--project", r.project)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return fmt.Errorf("reconcile Incus-managed Btrfs loop pool %q mount options: %s: %w", name, reason, err)
		}
		return fmt.Errorf("reconcile Incus-managed Btrfs loop pool %q mount options: %w", name, err)
	}

	verified, err := r.runner.Run(ctx, "incus", "storage", "get", name, "btrfs.mount_options", "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify Incus-managed Btrfs loop pool %q mount options: %w", name, err)
	}
	if got := strings.TrimSpace(verified.Stdout); got != desired {
		return fmt.Errorf("Incus-managed Btrfs loop pool %q mount options are %q after reconciliation, want %q", name, got, desired)
	}
	return nil
}
