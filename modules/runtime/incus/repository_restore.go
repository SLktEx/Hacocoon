package incus

import (
	"context"
	"encoding/json"
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

func (b *RepositoryBackend) DeleteRestoredWorkspaceVolume(ctx context.Context, target gitrepo.Object) error {
	if target.Kind != "work" {
		return core.ErrInvalidArgument
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return err
	}
	observe := func() (bool, error) {
		result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project+"&recursion=1")
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			return false, core.ErrRuntimeUnavailable
		}
		var volumes []persistentVolumeObservation
		if json.Unmarshal([]byte(result.Stdout), &volumes) != nil || volumes == nil {
			return false, core.ErrIncompatibleState
		}
		found := false
		for _, v := range volumes {
			if v.Name != name {
				continue
			}
			if found || v.Type != "custom" || v.ContentType != "filesystem" {
				return false, core.ErrIncompatibleState
			}
			for k, want := range volumeConfig(target) {
				if v.Config[k] != want {
					return false, core.ErrCapabilityStale
				}
			}
			if len(v.UsedBy) != 0 {
				return false, core.ErrStorageBusy
			}
			found = true
		}
		return found, nil
	}
	present, err := observe()
	if err != nil || !present {
		return err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "storage", "volume", "delete", pool, name, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	present, err = observe()
	if err != nil {
		return err
	}
	if present {
		return core.ErrRecoveryRequired
	}
	return nil
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
