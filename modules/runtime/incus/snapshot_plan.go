package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// PlanSnapshot enumerates the entire supported stopped aggregate without writes.
// The caller must hold canonical Environment/Workspace locks through capture.
func (r *Runtime) PlanSnapshot(ctx context.Context, source core.SnapshotSource, id string) ([]core.SnapshotComponent, error) {
	if !strings.HasPrefix(id, "snap-") || len(id) != 37 || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: strings.TrimPrefix(id, "snap-")}) || !core.ValidEnvironmentInstanceID(source.InstanceID) || !strings.HasPrefix(source.Environment.Workspace.Path, "managed:") || source.Environment.RuntimeRef == trustedHostName {
		return nil, core.ErrInvalidArgument
	}
	if validateManagedInstanceRef(source.Environment.RuntimeRef) != nil || r.managedWorkspace == nil {
		return nil, core.ErrUnsupported
	}
	mounts, err := r.managedWorkspace(ctx, source.Environment.Workspace.Path)
	if err != nil {
		return nil, err
	}
	if len(mounts) == 0 || len(mounts) > 253 {
		return nil, core.ErrIncompatibleState
	}
	mounts = append([]WorkspaceAttachment(nil), mounts...)
	sort.Slice(mounts, func(i, j int) bool { return mounts[i].Device < mounts[j].Device })
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/instances?project="+r.project+"&recursion=1")
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var list []snapshotInstanceObservation
	if json.Unmarshal([]byte(out.Stdout), &list) != nil || list == nil {
		return nil, core.ErrIncompatibleState
	}
	var instance *snapshotInstanceObservation
	for _, v := range list {
		if v.Name != source.Environment.RuntimeRef {
			continue
		}
		if instance != nil {
			return nil, core.ErrIncompatibleState
		}
		copy := v
		instance = &copy
	}
	if instance == nil {
		return nil, core.ErrNotFound
	}
	pool := ""
	for _, d := range instance.ExpandedDevices {
		if d["type"] == "disk" && d["path"] == "/" {
			if pool != "" || !safeIncusRef(d["pool"]) {
				return nil, core.ErrIncompatibleState
			}
			pool = d["pool"]
		}
	}
	owner := func() (string, error) {
		var value [16]byte
		_, err := rand.Read(value[:])
		return hex.EncodeToString(value[:]), err
	}
	rootOwner, err := owner()
	if err != nil {
		return nil, err
	}
	root := snapshotRootfsPlan{Pool: pool, Source: source.Environment.RuntimeRef, SourceInstanceID: source.InstanceID, Owner: rootOwner}
	verified, err := r.snapshotRootfsObservation(ctx, root, false)
	if err != nil {
		return nil, err
	}
	if verified == nil {
		return nil, core.ErrNotFound
	}
	// Use the second, ownership-verified device inventory throughout the plan.
	instance = verified
	out, err = r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var storage struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &storage) != nil || storage.Name != pool || storage.Driver != "btrfs" {
		return nil, core.ErrIncompatibleState
	}
	components := []core.SnapshotComponent{}
	add := func(b snapshotBinding) error {
		b.Version, b.Project = 1, r.project
		c, err := r.snapshotComponent(b)
		if err == nil {
			components = append(components, c)
		}
		return err
	}
	if err := add(snapshotBinding{Rootfs: &root}); err != nil {
		return nil, err
	}
	expected := map[string]bool{}
	volumes := map[string]bool{}
	addVolume := func(p snapshotVolumePlan) error {
		if p.Pool != pool || expected[p.Device] || volumes[p.Source] {
			return core.ErrIncompatibleState
		}
		d := instance.ExpandedDevices[p.Device]
		if d["type"] != "disk" || d["path"] != p.Path || d["pool"] != p.Pool || d["source"] != p.Source {
			return core.ErrCapabilityStale
		}
		for k, v := range d {
			if k != "type" && k != "path" && k != "pool" && k != "source" && !(k == "readonly" && (v == "true" || v == "false")) {
				return core.ErrUnsupported
			}
		}
		found, err := r.snapshotVolumeObservation(ctx, p, false)
		if err != nil {
			return err
		}
		if found == nil {
			return core.ErrNotFound
		}
		expected[p.Device], volumes[p.Source] = true, true
		return add(snapshotBinding{Volume: &p})
	}
	for _, m := range mounts {
		if !validWorkspaceAttachment(m) || !gitrepo.ValidID(m.Repository) {
			return nil, core.ErrIncompatibleState
		}
		targetOwner, err := owner()
		if err != nil {
			return nil, err
		}
		member := strings.TrimPrefix(m.Volume, "haco-work-")
		p := snapshotVolumePlan{Pool: m.Pool, Source: m.Volume, SourceOwner: m.Owner, SourceKind: "work", SourceID: m.Repository, SourceInstance: root.Source, SourceInstanceID: root.SourceInstanceID, Owner: targetOwner, Role: "workspace:" + member, Device: m.Device, Path: m.Path}
		if err := addVolume(p); err != nil {
			return nil, err
		}
	}
	attachment := source.Environment.PersistentResource
	if attachment != (core.PersistentResourceRef{}) {
		if !core.ValidPersistentResourceRef(attachment) {
			return nil, core.ErrInvalidArgument
		}
		targetOwner, err := owner()
		if err != nil {
			return nil, err
		}
		p := snapshotVolumePlan{Pool: pool, Source: "haco-persistent-" + attachment.Owner, SourceOwner: attachment.Owner, SourceKind: OCIStoreKind, SourceID: attachment.ID, SourceInstance: root.Source, SourceInstanceID: root.SourceInstanceID, Owner: targetOwner, Role: "oci", Device: "persistent-resource", Path: OCIStorePath}
		if err := addVolume(p); err != nil {
			return nil, err
		}
	}
	for name, d := range instance.ExpandedDevices {
		if d["type"] == "disk" && d["path"] != "/" && !expected[name] {
			return nil, core.ErrUnsupported
		}
		if d["type"] != "disk" && d["type"] != "nic" && d["type"] != "proxy" && d["type"] != "none" {
			return nil, core.ErrUnsupported
		}
	}
	return components, nil
}
func (r *Runtime) CreateSnapshotComponent(ctx context.Context, s core.SnapshotSource, c core.SnapshotComponent) error {
	return r.createSnapshotComponent(ctx, s, c)
}
func (r *Runtime) VerifySnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	return r.verifySnapshotComponent(ctx, c)
}
func (r *Runtime) DeleteSnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	return r.deleteSnapshotComponent(ctx, c)
}
