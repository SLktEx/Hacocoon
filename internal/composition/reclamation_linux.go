//go:build linux && (amd64 || arm64)

package composition

import (
	"context"

	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

// PrepareStorageReclamation selects the installation's pool, never a client path
// or pool name. It does not perform discard or grant outer-WSL authority.
func (a *App) PrepareStorageReclamation(ctx context.Context) (*incus.StorageReclamation, error) {
	storage := defaultIncusStorageAttachment()
	return a.Runtime.PrepareStorageReclamation(ctx, incus.BtrfsLoopPoolSpec{
		Name: storage["incus_pool"], MountOptions: storage["btrfs.mount_options"],
	})
}
