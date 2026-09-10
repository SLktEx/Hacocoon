//go:build linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"strings"
)

// ExportSnapshotComponent chooses the native Incus archive form after canonical
// routing. It requires the same saved-source reservation as the component methods.
func (r *Runtime) ExportSnapshotComponent(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (environmenttransfer.Archive, error) {
	var archive *NativeArchive
	var err error
	switch {
	case c.Role == "rootfs":
		archive, err = r.ExportSnapshotRootfs(ctx, c, root, limit)
	case c.Role == "oci" || strings.HasPrefix(c.Role, "workspace:"):
		archive, err = r.ExportSnapshotVolume(ctx, c, root, limit)
	default:
		return nil, core.ErrInvalidArgument
	}
	if archive == nil {
		return nil, err
	}
	return archive, err
}
