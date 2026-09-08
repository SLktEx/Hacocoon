package incus

import (
	"context"
	"encoding/json"
	"strings"

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
func (r *Runtime) snapshotBaseObservation(ctx context.Context, p snapshotBasePlan) (*snapshotInstanceObservation, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/instances?project="+r.project+"&recursion=1")
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var instances []snapshotInstanceObservation
	if json.Unmarshal([]byte(out.Stdout), &instances) != nil || instances == nil {
		return nil, core.ErrIncompatibleState
	}
	var found *snapshotInstanceObservation
	expected := p.config()
	fingerprint, _ := baseRevisionFingerprint(p.Base.Revision)
	for _, instance := range instances {
		if instance.Name != p.target() {
			continue
		}
		if found != nil || instance.Type != "container" || strings.ToUpper(instance.Status) != "STOPPED" || instance.Ephemeral || len(instance.Profiles) != 0 || instance.Config == nil || instance.ExpandedConfig == nil {
			return nil, core.ErrIncompatibleState
		}
		for key, value := range expected {
			if instance.Config[key] != value || instance.ExpandedConfig[key] != value {
				return nil, core.ErrCapabilityStale
			}
		}
		if instance.Config["volatile.base_image"] != fingerprint || instance.ExpandedConfig["volatile.base_image"] != fingerprint {
			return nil, core.ErrCapabilityStale
		}
		for _, config := range []map[string]string{instance.Config, instance.ExpandedConfig} {
			for key, value := range config {
				// Incus stores image properties under image.*; these are metadata, not
				// workload config. Neither image properties nor volatile data grant ownership.
				if value == "" || strings.HasPrefix(key, "volatile.") || strings.HasPrefix(key, "image.") {
					continue
				}
				if expected[key] != value {
					return nil, core.ErrIncompatibleState
				}
			}
		}
		for _, devices := range []map[string]map[string]string{instance.Devices, instance.ExpandedDevices} {
			root := devices["root"]
			if len(devices) != 1 || len(root) != 3 || root["type"] != "disk" || root["path"] != "/" || root["pool"] != p.Pool {
				return nil, core.ErrIncompatibleState
			}
		}
		copy := instance
		found = &copy
	}
	return found, nil
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
	found, err := r.snapshotBaseObservation(ctx, p)
	if err != nil {
		return err
	}
	if found == nil {
		return nil
	}
	out, err := r.runner.Run(ctx, "incus", "delete", p.target(), "--project", r.project)
	if err != nil || out.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	found, err = r.snapshotBaseObservation(ctx, p)
	if err != nil {
		return err
	}
	if found != nil {
		return core.ErrRecoveryRequired
	}
	return nil
}
