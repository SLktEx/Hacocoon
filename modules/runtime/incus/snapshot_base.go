package incus

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// snapshotBasePlan retains an independent rootfs for the exact effective Base.
// The aggregate coordinator must persist this binding before creation.
type snapshotBasePlan struct {
	Pool, Owner string
	Base        core.BaseRef
	Asset       *core.BaseAsset `json:"asset,omitempty"`
}

func (p snapshotBasePlan) target() string { return "haco-snapshot-base-" + p.Owner }
func (p snapshotBasePlan) validate() error {
	if !safeIncusRef(p.Pool) || validateBaseName(p.Base.Name) != nil || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: p.Owner}) {
		return core.ErrInvalidArgument
	}
	_, err := baseRevisionFingerprint(p.Base.Revision)
	return err
}
func (p snapshotBasePlan) config() map[string]string {
	return map[string]string{"user.hacocoon.kind": "snapshot-base", "user.hacocoon.owner": p.Owner,
		"user.hacocoon.base-name": string(p.Base.Name), "user.hacocoon.base-revision": string(p.Base.Revision),
		"boot.autostart": "false", "security.privileged": "false", "security.nesting": "false"}
}
func (r *Runtime) createSnapshotBase(ctx context.Context, p snapshotBasePlan) error {
	if err := p.validate(); err != nil {
		return err
	}
	source, err := r.snapshotBaseSource(ctx, p)
	if err != nil {
		return err
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var pool struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &pool) != nil || pool.Name != p.Pool || pool.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	data, err := json.Marshal(map[string]any{"name": p.target(), "type": "container", "ephemeral": false,
		"profiles": []string{}, "config": p.config(), "devices": map[string]any{"root": map[string]string{"type": "disk", "path": "/", "pool": p.Pool}},
		"source": source})
	if err != nil {
		return err
	}
	out, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/instances?project="+r.project, "--data", string(data))
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	// Record the exact created identity before any further provider operation.
	return nil
}
func (p snapshotBasePlan) storageIdentity() baseStorageIdentity {
	return baseStorageIdentity{Kind: "snapshot-base", Pool: p.Pool, Owner: p.Owner, Base: p.Base}
}
func (r *Runtime) snapshotBaseObservation(ctx context.Context, p snapshotBasePlan) (*snapshotInstanceObservation, error) {
	return r.baseStorageObservation(ctx, p.storageIdentity())
}
func (r *Runtime) verifySnapshotBase(ctx context.Context, p snapshotBasePlan) error {
	found, err := r.snapshotBaseObservation(ctx, p)
	if err != nil {
		return err
	}
	if found == nil {
		return core.ErrNotFound
	}
	return nil
}
func (r *Runtime) deleteSnapshotBase(ctx context.Context, p snapshotBasePlan) error {
	return r.deleteBaseStorage(ctx, p.storageIdentity())
}

func (r *Runtime) snapshotBaseSource(ctx context.Context, p snapshotBasePlan) (map[string]any, error) {
	if p.Asset != nil {
		identity, err := r.decodeSnapshotBaseAsset(p)
		if err != nil {
			return nil, err
		}
		a := *p.Asset
		backend := &BaseAssetBackend{Provider: &BaseProvider{Runtime: r}}
		if err := backend.Verify(ctx, a); err != nil {
			return nil, err
		}
		return map[string]any{"type": "copy", "source": identity.target(), "project": r.project, "instance_only": true, "live": false}, nil
	}
	fingerprint, err := baseRevisionFingerprint(p.Base.Revision)
	if err != nil {
		return nil, err
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/images/"+fingerprint+"?project="+r.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var image struct{ Fingerprint, Type string }
	if json.Unmarshal([]byte(out.Stdout), &image) != nil || image.Fingerprint != fingerprint || image.Type != "container" {
		return nil, core.ErrIncompatibleState
	}
	return map[string]any{"type": "image", "fingerprint": fingerprint}, nil
}

func (r *Runtime) decodeSnapshotBaseAsset(p snapshotBasePlan) (baseStorageIdentity, error) {
	a := *p.Asset
	if a.Base != p.Base || a.Scope != r.project+"/"+p.Pool || a.State != "ready" || a.Owner == p.Owner {
		return baseStorageIdentity{}, core.ErrCapabilityStale
	}
	identity, _, err := (&BaseAssetBackend{Provider: &BaseProvider{Runtime: r}}).decode(a)
	return identity, err
}
