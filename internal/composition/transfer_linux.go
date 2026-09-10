//go:build linux

package composition

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

// ExportEnvironment composes the existing local Incus implementation. root is a
// private controller-owned staging directory, never a client-provided Host path.
// Public transport must authorize the request and publish only a successful result.
func (a *App) ExportEnvironment(ctx context.Context, source, root string, limit int64) (environmenttransfer.ExportResult, error) {
	if a == nil || a.Environments == nil || a.Runtime == nil {
		return environmenttransfer.ExportResult{}, core.ErrUnsupported
	}
	exporter := environmenttransfer.Exporter{Snapshots: a.Environments, Root: root,
		Component: func(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (environmenttransfer.Archive, error) {
			// Keep native image/volume decisions out of the transfer envelope package.
			switch {
			case c.Role == "rootfs":
				archive, err := a.Runtime.ExportSnapshotRootfs(ctx, c, root, limit)
				if archive == nil {
					return nil, err
				}
				return archive, err
			case c.Role == "oci" || strings.HasPrefix(c.Role, "workspace:"):
				archive, err := a.Runtime.ExportSnapshotVolume(ctx, c, root, limit)
				if archive == nil {
					return nil, err
				}
				return archive, err
			default:
				return nil, core.ErrInvalidArgument
			}
		},
	}
	return exporter.ExportStopped(ctx, source, limit)
}
