//go:build linux

package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"sort"
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

// ExportSnapshotWorkspaces resolves the same immutable bindings as native export.
// The caller holds the snapshot read reservation through metadata and byte export.
func (r *Runtime) ExportSnapshotWorkspaces(_ context.Context, saved core.Snapshot) ([]environmenttransfer.Workspace, error) {
	if saved.State != "ready" {
		return nil, core.ErrIncompatibleState
	}
	components := append([]core.SnapshotComponent(nil), saved.Components...)
	sort.Slice(components, func(i, j int) bool { return components[i].Role < components[j].Role })
	out := []environmenttransfer.Workspace{}
	seen := map[string]bool{}
	for _, c := range components {
		if !strings.HasPrefix(c.Role, "workspace:") {
			continue
		}
		plan, err := r.decodeSavedComponent(c)
		if err != nil {
			return nil, err
		}
		p := plan.Volume
		if p == nil || p.SourceKind != "work" || c.State != "verified" || seen[p.SourceID] {
			return nil, core.ErrIncompatibleState
		}
		seen[p.SourceID] = true
		role := "workspace"
		if len(out) > 0 {
			role = fmt.Sprintf("workspace-%03d", len(out)+1)
		}
		out = append(out, environmenttransfer.Workspace{Role: role, Name: p.SourceID, Remote: p.Remote, Branch: p.Branch})
	}
	if len(out) == 0 {
		return nil, core.ErrIncompatibleState
	}
	return out, nil
}
