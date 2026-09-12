//go:build linux

package composition

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"io"
)

// ExportEnvironment composes canonical ownership and the configured Incus route.
// root is a private controller-owned staging directory, never a client Host path.
// Public transport must authorize the request and publish only a successful result.
func (a *App) ExportEnvironment(ctx context.Context, source, root string, limit int64) (environmenttransfer.ExportResult, error) {
	if a == nil || a.Environments == nil || a.Bases == nil {
		return environmenttransfer.ExportResult{}, core.ErrUnsupported
	}
	exporter := environmenttransfer.Exporter{Snapshots: a.Environments, Root: root, Component: a.Bases.ExportSnapshotComponent, Workspaces: a.Bases.ExportSnapshotWorkspaces}
	return exporter.ExportStopped(ctx, source, limit)
}

// ImportEnvironment reuses the configured retained-data owners and canonical Env lifecycle.
func (a *App) ImportEnvironment(ctx context.Context, source io.Reader, name, root string) (environmenttransfer.ImportResult, error) {
	if a == nil || a.transferCatalog == nil || a.Environments == nil || a.Repositories == nil || a.PersistentResources == nil {
		return environmenttransfer.ImportResult{}, core.ErrUnsupported
	}
	importer := environmenttransfer.Importer{Catalog: a.transferCatalog, Environments: a.Environments, Workspaces: a.Repositories, Stores: a.PersistentResources, Root: root, StoreKind: ociplugin.StoreKind}
	return importer.Import(ctx, source, name, environmenttransfer.DefaultPayloadLimit)
}
