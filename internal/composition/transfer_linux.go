//go:build linux

package composition

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

// ExportEnvironment composes canonical ownership and the configured Incus route.
// root is a private controller-owned staging directory, never a client Host path.
// Public transport must authorize the request and publish only a successful result.
func (a *App) ExportEnvironment(ctx context.Context, source, root string, limit int64) (environmenttransfer.ExportResult, error) {
	if a == nil || a.Environments == nil || a.Bases == nil {
		return environmenttransfer.ExportResult{}, core.ErrUnsupported
	}
	exporter := environmenttransfer.Exporter{Snapshots: a.Environments, Root: root, Component: a.Bases.ExportSnapshotComponent}
	return exporter.ExportStopped(ctx, source, limit)
}
