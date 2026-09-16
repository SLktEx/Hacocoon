package composition

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/adapters/incus"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

func (a *App) DiagnoseHost(ctx context.Context) (diagnostics.Report, error) {
	storage := defaultIncusStorageAttachment()
	return a.Runtime.DiagnoseHost(ctx, incus.BtrfsLoopPoolSpec{
		Name: storage["incus_pool"], MountOptions: storage["btrfs.mount_options"],
	})
}
