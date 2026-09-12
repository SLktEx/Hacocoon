package incus

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Only storage identities belong here. Workload config and host credentials are
// deliberately not representable. This binding is persisted before any create.
type snapshotBinding struct {
	Version int                 `json:"version"`
	Project string              `json:"project"`
	Rootfs  *snapshotRootfsPlan `json:"rootfs,omitempty"`
	Volume  *snapshotVolumePlan `json:"volume,omitempty"`
	Base    *snapshotBasePlan   `json:"base,omitempty"`
}

func (r *Runtime) snapshotComponent(binding snapshotBinding) (core.SnapshotComponent, error) {
	if binding.Version != 1 || binding.Project != r.project {
		return core.SnapshotComponent{}, core.ErrInvalidArgument
	}
	c := core.SnapshotComponent{State: "planned"}
	switch {
	case binding.Rootfs != nil && binding.Volume == nil && binding.Base == nil:
		p := *binding.Rootfs
		if err := p.validate(); err != nil {
			return c, err
		}
		c.Role, c.Owner, c.NativeRef = "rootfs", p.Owner, "instance/"+p.target()
	case binding.Volume != nil && binding.Rootfs == nil && binding.Base == nil:
		p := *binding.Volume
		if err := p.validate(); err != nil {
			return c, err
		}
		c.Role, c.Owner, c.NativeRef = p.Role, p.Owner, "volume/"+p.Pool+"/"+p.target()
	case binding.Base != nil && binding.Rootfs == nil && binding.Volume == nil:
		p := *binding.Base
		if err := p.validate(); err != nil {
			return c, err
		}
		if p.Asset != nil {
			if _, err := r.decodeSnapshotBaseAsset(p); err != nil {
				return c, err
			}
		}
		c.Role, c.Owner, c.NativeRef = "base", p.Owner, "instance/"+p.target()
	default:
		return c, core.ErrInvalidArgument
	}
	data, err := json.Marshal(binding)
	if err != nil || len(data) > 16384 {
		return c, core.ErrInvalidArgument
	}
	c.Binding = string(data)
	return c, nil
}
func (r *Runtime) decodeSnapshotComponent(c core.SnapshotComponent) (snapshotBinding, error) {
	var binding snapshotBinding
	if c.Binding == "" || len(c.Binding) > 16384 || json.Unmarshal([]byte(c.Binding), &binding) != nil {
		return binding, core.ErrInvalidArgument
	}
	expected, err := r.snapshotComponent(binding)
	if err != nil {
		return binding, err
	}
	expected.State = c.State
	// Canonical re-encoding also rejects unknown/duplicate fields and alternate
	// spellings. The outer target/owner/role must agree with the complete plan.
	if expected != c {
		return binding, core.ErrCapabilityStale
	}
	return binding, nil
}
func (r *Runtime) createSnapshotComponent(ctx context.Context, source core.SnapshotSource, c core.SnapshotComponent) error {
	b, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return err
	}
	if c.State != "planned" {
		return core.ErrIncompatibleState
	}
	switch {
	case b.Rootfs != nil:
		if b.Rootfs.Source != source.Environment.RuntimeRef || b.Rootfs.SourceInstanceID != source.InstanceID {
			return core.ErrCapabilityStale
		}
		return r.createSnapshotRootfs(ctx, *b.Rootfs)
	case b.Volume != nil:
		if b.Volume.SourceInstance != source.Environment.RuntimeRef || b.Volume.SourceInstanceID != source.InstanceID {
			return core.ErrCapabilityStale
		}
		if b.Volume.Role == "oci" && (b.Volume.SourceID != source.Environment.PersistentResource.ID || b.Volume.SourceOwner != source.Environment.PersistentResource.Owner) {
			return core.ErrCapabilityStale
		}
		return r.createSnapshotVolume(ctx, *b.Volume)
	case b.Base != nil:
		if source.Environment.Base == nil || !reflect.DeepEqual(*source.Environment.Base, b.Base.Base) {
			return core.ErrCapabilityStale
		}
		return r.createSnapshotBase(ctx, *b.Base)
	}
	return core.ErrUnsupported
}
func (r *Runtime) verifySnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	b, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return err
	}
	if c.State != "created" && c.State != "verified" {
		return core.ErrIncompatibleState
	}
	switch {
	case b.Rootfs != nil:
		return r.verifySnapshotRootfs(ctx, *b.Rootfs)
	case b.Volume != nil:
		return r.verifySnapshotVolume(ctx, *b.Volume)
	case b.Base != nil:
		return r.verifySnapshotBase(ctx, *b.Base)
	}
	return core.ErrUnsupported
}
func (r *Runtime) deleteSnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	b, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return err
	}
	if c.State != "planned" && c.State != "created" && c.State != "verified" {
		return core.ErrIncompatibleState
	}
	switch {
	case b.Rootfs != nil:
		return r.deleteSnapshotRootfs(ctx, *b.Rootfs)
	case b.Volume != nil:
		return r.deleteSnapshotVolume(ctx, *b.Volume)
	case b.Base != nil:
		return r.deleteSnapshotBase(ctx, *b.Base)
	}
	return core.ErrUnsupported
}
