package composition

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

func (a *App) DiagnoseHost(ctx context.Context) (diagnostics.Report, error) {
	storage := defaultIncusStorageAttachment()
	return a.Runtime.DiagnoseHost(ctx, incus.BtrfsLoopPoolSpec{
		Name: storage["incus_pool"], MountOptions: storage["btrfs.mount_options"],
	})
}
