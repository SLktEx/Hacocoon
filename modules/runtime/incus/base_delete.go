package incus

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func validBaseImageIdentity(id basemanage.Identity) bool {
	return basebuild.NamePattern.MatchString(string(id.Name)) && baseFingerprintPattern.MatchString(id.Fingerprint) && core.ValidEnvironmentInstanceID(id.BuildInstance)
}
func (p *BaseProvider) ListBaseImages(ctx context.Context) ([]basemanage.Image, error) {
	p.baseMu.RLock()
	defer p.baseMu.RUnlock()
	return p.listBaseImages(ctx)
}
func (p *BaseProvider) listBaseImages(ctx context.Context) ([]basemanage.Image, error) {
	var images []baseImage
	if err := p.baseQuery(ctx, "GET", p.basePath("")+"&recursion=1", nil, &images); err != nil {
		return nil, err
	}
	if images == nil {
		return nil, core.ErrIncompatibleState
	}
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return nil, err
	}
	var instances []snapshotInstanceObservation
	if err := p.baseQuery(ctx, "GET", "/1.0/instances?project="+url.QueryEscape(p.project)+"&recursion=1", nil, &instances); err != nil {
		return nil, err
	}
	if instances == nil {
		return nil, core.ErrIncompatibleState
	}
	result := []basemanage.Image{}
	seen := map[string]bool{}
	for _, image := range images {
		if !baseFingerprintPattern.MatchString(image.Fingerprint) {
			return nil, core.ErrIncompatibleState
		}
		if seen[image.Fingerprint] {
			return nil, core.ErrIncompatibleState
		}
		seen[image.Fingerprint] = true
		if image.Properties["user.hacocoon.kind"] != "base-image" {
			continue
		}
		id := basemanage.Identity{Name: core.BaseName(image.Properties["user.hacocoon.base-name"]), Fingerprint: image.Fingerprint, BuildInstance: image.Properties["user.hacocoon.build-instance"]}
		if !validBaseImageIdentity(id) || image.Public || image.Type != "container" {
			return nil, core.ErrCapabilityStale
		}
		v := basemanage.Image{Identity: id, Aliases: []string{}, NativeUsers: []string{}, ProtectedAliases: []string{}}
		for _, a := range aliases {
			if a.Target != id.Fingerprint {
				continue
			}
			v.Aliases = append(v.Aliases, a.Name)
			current := a.Name == builtBasePrefix+string(id.Name)
			if current {
				v.Current = true
			}
			for name, source := range p.sources {
				if source == a.Name || source == "local:"+a.Name {
					v.ProtectedAliases = append(v.ProtectedAliases, "configured:"+string(name))
				}
			}
			if a.Description != builtBaseDescription || a.Type != "container" || (!current && a.Name != "hacocoon-build-"+id.BuildInstance) {
				v.ProtectedAliases = append(v.ProtectedAliases, a.Name)
			}
		}
		for name, source := range p.sources {
			if source == "local:"+id.Fingerprint || source == id.Fingerprint {
				v.ProtectedAliases = append(v.ProtectedAliases, "configured:"+string(name))
			}
		}
		for _, instance := range instances {
			if instance.Config == nil || instance.Name == "" {
				return nil, core.ErrIncompatibleState
			}
			if instance.Config["volatile.base_image"] != id.Fingerprint {
				continue
			}
			// Independent saved rootfs/Base instances remain after image deletion. Their
			// native data are never deleted or rewritten by this operation.
			owner := instance.Config["user.hacocoon.owner"]
			kind := instance.Config["user.hacocoon.kind"]
			saved := core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: owner}) && ((kind == "snapshot-rootfs" && instance.Name == "haco-snapshot-root-"+owner) || ((kind == "base" || kind == "snapshot-base") && instance.Name == "haco-"+kind+"-"+owner))
			if !saved {
				v.NativeUsers = append(v.NativeUsers, instance.Name)
			}
		}
		sort.Strings(v.Aliases)
		sort.Strings(v.NativeUsers)
		sort.Strings(v.ProtectedAliases)
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Fingerprint < result[j].Fingerprint
	})
	return result, nil
}

// DeleteBaseImage accepts the reviewed immutable fingerprint and publication
// owner. It never guesses an image from a mutable alias or deletes saved instances.
func (p *BaseProvider) DeleteBaseImage(ctx context.Context, id basemanage.Identity, recheck func(context.Context) error) error {
	if !validBaseImageIdentity(id) || recheck == nil {
		return core.ErrInvalidArgument
	}
	p.baseMu.Lock()
	defer p.baseMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := recheck(ctx); err != nil {
		return err
	}
	images, err := p.listBaseImages(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, v := range images {
		if v.Fingerprint == id.Fingerprint {
			if v.Identity != id {
				return core.ErrCapabilityStale
			}
			if len(v.NativeUsers) > 0 || len(v.ProtectedAliases) > 0 {
				return core.ErrStorageBusy
			}
			found = true
		}
	}
	if !found {
		return core.ErrNotFound
	}
	if err := p.baseQuery(ctx, "DELETE", p.basePath("/"+id.Fingerprint), nil, nil); err != nil {
		return core.ErrRecoveryRequired
	}
	// Observe the complete collection independently; do not turn a failed query
	// into absence or drop any Hacocoon catalog record on uncertainty.
	var remaining []baseImage
	if err := p.baseQuery(ctx, "GET", p.basePath("")+"&recursion=1", nil, &remaining); err != nil {
		return err
	}
	if remaining == nil {
		return core.ErrIncompatibleState
	}
	for _, image := range remaining {
		if !baseFingerprintPattern.MatchString(image.Fingerprint) {
			return core.ErrIncompatibleState
		}
		if strings.EqualFold(image.Fingerprint, id.Fingerprint) {
			return core.ErrRecoveryRequired
		}
	}
	return nil
}
