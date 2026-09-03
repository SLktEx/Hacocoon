package composition

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

func ensureDefaultIncusStoragePool(ctx context.Context, runner host.Runner) (map[string]string, error) {
	attachment := defaultIncusStorageAttachment()
	pool, err := incus.New(runner).EnsureBtrfsLoopPool(ctx, incus.BtrfsLoopPoolSpec{
		Name:         attachment["incus_pool"],
		Size:         attachment["size"],
		MountOptions: attachment["btrfs.mount_options"],
	})
	if err != nil {
		return nil, err
	}

	// Return only the pool identity. The generic runtime path immediately
	// verifies it and does not need a Host source path because Incus owns the
	// sparse image, loop device and mount lifecycle.
	return map[string]string{"incus_pool": pool}, nil
}
