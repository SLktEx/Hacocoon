package incus

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func (b *RepositoryBackend) SavedWorkspaces(ctx context.Context, saved core.Snapshot) ([]gitrepo.SavedWorkspace, error) {
	if saved.State != "ready" {
		return nil, core.ErrIncompatibleState
	}
	var out []gitrepo.SavedWorkspace
	seen := map[string]bool{}
	for _, c := range saved.Components {
		if !strings.HasPrefix(c.Role, "workspace:") {
			continue
		}
		plan, err := b.Runtime.decodeSavedComponent(c)
		if err != nil {
			return nil, err
		}
		p := plan.Volume
		if p == nil || p.SourceKind != "work" || c.State != "verified" {
			return nil, core.ErrIncompatibleState
		}
		// Old manifests remain readable/deletable. They cannot invent missing trusted
		// Git routing metadata from mutable guest files or a recycled registry name.
		if p.Remote == "" || p.Branch == "" {
			return nil, core.ErrUnsupported
		}
		if seen[p.SourceID] {
			return nil, core.ErrIncompatibleState
		}
		seen[p.SourceID] = true
		if err := b.Runtime.verifySnapshotVolume(ctx, *p); err != nil {
			return nil, err
		}
		out = append(out, gitrepo.SavedWorkspace{Component: c, Repository: p.SourceID, Remote: p.Remote, Branch: p.Branch})
	}
	if len(out) < 1 || len(out) > 8 {
		return nil, core.ErrIncompatibleState
	}
	for _, source := range out {
		plan, _ := b.Runtime.decodeSavedComponent(source.Component)
		p := plan.Volume
		device, path := "workspace", "/workspace"
		if len(out) > 1 {
			device += "-" + p.SourceID
			path += "/" + p.SourceID
		}
		if p.Device != device || p.Path != path {
			return nil, core.ErrIncompatibleState
		}
	}
	return out, nil
}

func (b *RepositoryBackend) CreateSavedWorkspace(ctx context.Context, target gitrepo.Object, source gitrepo.SavedWorkspace) error {
	plan, err := b.Runtime.decodeSavedComponent(source.Component)
	if err != nil {
		return err
	}
	p := plan.Volume
	if p == nil || p.SourceKind != "work" || source.Component.State != "verified" || target.Kind != "work" ||
		target.Repository != p.SourceID || target.Remote != p.Remote || target.Branch != p.Branch ||
		source.Repository != p.SourceID || source.Remote != p.Remote || source.Branch != p.Branch ||
		p.Remote == "" || target.Owner == p.Owner || target.Owner == p.SourceOwner {
		return core.ErrCapabilityStale
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return err
	}
	if pool != p.Pool || name == p.target() || name == p.Source {
		return core.ErrIncompatibleState
	}
	return b.Runtime.copySavedVolume(ctx, p, pool, name, volumeConfig(target))
}

// Restore stays in the saved pool; the current default profile/pool is unrelated
// to the owned saved volume and must not become a new restore dependency.
func (b *RepositoryBackend) PlanSavedWorkspace(_ context.Context, id string, source gitrepo.SavedWorkspace) (string, error) {
	if !gitrepo.ValidID(id) {
		return "", core.ErrInvalidArgument
	}
	binding, err := b.Runtime.decodeSavedComponent(source.Component)
	if err != nil {
		return "", err
	}
	p := binding.Volume
	if p == nil || p.SourceKind != "work" || source.Component.State != "verified" {
		return "", core.ErrIncompatibleState
	}
	return p.Pool + "/haco-work-" + id, nil
}
