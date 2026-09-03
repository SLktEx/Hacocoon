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
