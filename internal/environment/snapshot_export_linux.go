//go:build linux

package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

// ExportSnapshotComponent keeps the catalog route and exact native ownership
// together. It never sends a wrapped catalog ref straight to a native adapter.
// The caller retains the canonical saved-source reservation throughout export.
func (r *Router) ExportSnapshotComponent(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (environmenttransfer.Archive, error) {
	backend, local, _, err := r.snapshotBackend(c)
	if err != nil {
		return nil, err
	}
	exporter, ok := backend.(interface {
		ExportSnapshotComponent(context.Context, core.SnapshotComponent, string, int64) (environmenttransfer.Archive, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	return exporter.ExportSnapshotComponent(ctx, local, root, limit)
}
