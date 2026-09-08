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
	fingerprint, _ := baseRevisionFingerprint(p.Base.Revision)
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/images/"+fingerprint+"?project="+r.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var image struct{ Fingerprint, Type string }
	if json.Unmarshal([]byte(out.Stdout), &image) != nil || image.Fingerprint != fingerprint || image.Type != "container" {
		return core.ErrIncompatibleState
	}
	out, err = r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var pool struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &pool) != nil || pool.Name != p.Pool || pool.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	data, err := json.Marshal(map[string]any{"name": p.target(), "type": "container", "ephemeral": false,
		"profiles": []string{}, "config": p.config(), "devices": map[string]any{"root": map[string]string{"type": "disk", "path": "/", "pool": p.Pool}},
		"source": map[string]string{"type": "image", "fingerprint": fingerprint}})
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
