package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/url"
)

type snapshotImagePlan struct {
	Name   string
	Owner  string
	Rootfs snapshotRootfsPlan
}

func (p snapshotImagePlan) validate() error {
	if core.ValidateEnvironmentName(p.Name) != nil || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "snapshot:image", Owner: p.Owner}) {
		return core.ErrInvalidArgument
	}
	return p.Rootfs.validate()
}
func (r *Runtime) checkSnapshotImageName(ctx context.Context, plan snapshotImagePlan) error {
	p := &BaseProvider{Runtime: r}
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		if alias.Name == builtBasePrefix+plan.Name {
			return core.ErrAlreadyExists
		}
	}
	return nil
}
func (r *Runtime) createSnapshotImage(ctx context.Context, plan snapshotImagePlan) error {
	if err := r.verifySnapshotRootfs(ctx, plan.Rootfs); err != nil {
		return err
	}
	if err := r.checkSnapshotImageName(ctx, plan); err != nil {
		return err
	}
	p := &BaseProvider{Runtime: r}
	// Publication includes ownership and alias atomically in Incus. The saved
	// stopped rootfs excludes the source's Workspace and OCI mounts.
	return p.baseQuery(ctx, "POST", p.basePath(""), map[string]any{
		"public": false, "auto_update": false,
		"properties": map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": plan.Name, "user.hacocoon.build-instance": plan.Rootfs.SourceInstanceID, "user.hacocoon.snapshot-owner": plan.Owner},
		"source":     map[string]string{"type": "instance", "name": plan.Rootfs.target()},
		"aliases":    []map[string]string{{"name": builtBasePrefix + plan.Name, "description": builtBaseDescription}},
	}, nil)
}
func (r *Runtime) snapshotImage(ctx context.Context, plan snapshotImagePlan) (core.BaseRef, error) {
	p := &BaseProvider{Runtime: r}
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return core.BaseRef{}, err
	}
	for _, alias := range aliases {
		if alias.Name != builtBasePrefix+plan.Name {
			continue
		}
		image, err := p.ownedBaseImage(ctx, core.BaseName(plan.Name), alias)
		if err != nil {
			return core.BaseRef{}, err
		}
		if image.Properties["user.hacocoon.snapshot-owner"] != plan.Owner {
			return core.BaseRef{}, core.ErrCapabilityStale
		}
		return core.BaseRef{Name: core.BaseName(plan.Name), Revision: core.BaseRevision("sha256:" + image.Fingerprint)}, nil
	}
	return core.BaseRef{}, core.ErrNotFound
}

// Inventory by the durable ownership token, not by a removable alias. Losing an
// alias must never be mistaken for successful provider resource deletion.
func (r *Runtime) ownedSnapshotImages(ctx context.Context, plan snapshotImagePlan) ([]baseImage, error) {
	p := &BaseProvider{Runtime: r}
	var all []baseImage
	if err := p.baseQuery(ctx, "GET", p.basePath("")+"&recursion=1", nil, &all); err != nil {
		return nil, err
	}
	if all == nil {
		return nil, core.ErrIncompatibleState
	}
	found := []baseImage{}
	for _, image := range all {
		if image.Properties["user.hacocoon.snapshot-owner"] != plan.Owner {
			continue
		}
		if !baseFingerprintPattern.MatchString(image.Fingerprint) || image.Public || image.Type != "container" || image.Properties["user.hacocoon.kind"] != "base-image" || image.Properties["user.hacocoon.base-name"] != plan.Name || image.Properties["user.hacocoon.build-instance"] != plan.Rootfs.SourceInstanceID {
			return nil, core.ErrCapabilityStale
		}
		found = append(found, image)
	}
	if len(found) > 1 {
		return nil, core.ErrRecoveryRequired
	}
	return found, nil
}
func (r *Runtime) deleteSnapshotImage(ctx context.Context, plan snapshotImagePlan) error {
	images, err := r.ownedSnapshotImages(ctx, plan)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return nil
	}
	p := &BaseProvider{Runtime: r}
	if err := p.baseQuery(ctx, "DELETE", p.basePath("/"+url.PathEscape(images[0].Fingerprint)), nil, nil); err != nil {
		return err
	}
	remaining, err := r.ownedSnapshotImages(ctx, plan)
	if err != nil || len(remaining) != 0 {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return nil
}
func (r *Runtime) SnapshotImage(ctx context.Context, component core.SnapshotComponent) (core.BaseRef, error) {
	binding, err := r.decodeSnapshotComponent(component)
	if err != nil {
		return core.BaseRef{}, err
	}
	if binding.Image == nil {
		return core.BaseRef{}, core.ErrInvalidArgument
	}
	return r.snapshotImage(ctx, *binding.Image)
}
