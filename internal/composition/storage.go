package composition

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultIncusProject = "hacocoon"

func ensureDefaultIncusStoragePool(ctx context.Context, runner host.Runner) (map[string]string, error) {
	attachment := defaultIncusStorageAttachment()
	pool := attachment["incus_pool"]
	if _, err := runner.Run(ctx, "incus", "storage", "show", pool, "--project", defaultIncusProject); err == nil {
		return map[string]string{"incus_pool": pool}, nil
	}

	args := []string{
		"storage", "create", pool, attachment["driver"],
		"size=" + attachment["size"],
		"btrfs.mount_options=" + attachment["btrfs.mount_options"],
		"--project", defaultIncusProject,
	}
	result, err := runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return nil, fmt.Errorf("create Incus-managed Btrfs storage pool %q: %s: %w", pool, reason, err)
		}
		return nil, fmt.Errorf("create Incus-managed Btrfs storage pool %q: %w", pool, err)
	}

	// Return only the pool identity. The runtime immediately verifies it with
	// `incus storage show`; it does not need a Host source path because Incus
	// owns the loop image and mount lifecycle for this pool.
	return map[string]string{"incus_pool": pool}, nil
}
