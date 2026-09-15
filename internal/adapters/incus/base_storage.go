package incus

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// baseStorageIdentity describes an isolated, never-runnable Base rootfs. The
// immutable asset and snapshot component have distinct ownership namespaces.
type baseStorageIdentity struct {
	Kind, Pool, Owner string
	Base              core.BaseRef
}

func (p baseStorageIdentity) target() string { return "haco-" + p.Kind + "-" + p.Owner }
func (p baseStorageIdentity) validate() error {
	if (p.Kind != "base" && p.Kind != "snapshot-base") || !safeIncusRef(p.Pool) || validateBaseName(p.Base.Name) != nil || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: p.Owner}) {
		return core.ErrInvalidArgument
	}
	_, err := baseRevisionFingerprint(p.Base.Revision)
	return err
}
func (p baseStorageIdentity) config() map[string]string {
	return map[string]string{"user.hacocoon.kind": p.Kind, "user.hacocoon.owner": p.Owner,
		"user.hacocoon.base-name": string(p.Base.Name), "user.hacocoon.base-revision": string(p.Base.Revision),
		"boot.autostart": "false", "security.privileged": "false", "security.nesting": "false"}
}
func (r *Runtime) baseStorageObservation(ctx context.Context, p baseStorageIdentity) (*snapshotInstanceObservation, error) {
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
func (r *Runtime) deleteBaseStorage(ctx context.Context, p baseStorageIdentity) error {
	found, err := r.baseStorageObservation(ctx, p)
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
	found, err = r.baseStorageObservation(ctx, p)
	if err != nil {
		return err
	}
	if found != nil {
		return core.ErrRecoveryRequired
	}
	return nil
}
