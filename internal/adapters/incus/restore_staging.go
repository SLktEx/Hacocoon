package incus

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Legacy staging ownership is decoded only for verification and cleanup.
// New staging creation is retired; new consumers use canonical Environment creation.
// The saved component is a complete immutable source ownership receipt.
type restoreBinding struct {
	Version int                    `json:"version"`
	Project string                 `json:"project"`
	Owner   string                 `json:"owner"`
	Source  core.SnapshotComponent `json:"source"`
}

func (b restoreBinding) target() string { return "haco-restore-" + b.Owner }
func (b restoreBinding) config(instance bool) map[string]string {
	out := map[string]string{"user.hacocoon.kind": "restore-staging", "user.hacocoon.owner": b.Owner, "user.hacocoon.restore-role": b.Source.Role}
	if instance {
		out["boot.autostart"] = "false"
		out["security.privileged"] = "false"
		out["security.nesting"] = "false"
	}
	return out
}
func (r *Runtime) restoreShape(b restoreBinding) (snapshotBinding, string, string, bool, error) {
	var source snapshotBinding
	if b.Version != 1 || b.Project != r.project || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:restore", Owner: b.Owner}) || b.Owner == b.Source.Owner || b.Source.State != "verified" {
		return source, "", "", false, core.ErrInvalidArgument
	}
	source, err := r.decodeSnapshotComponent(b.Source)
	if err != nil {
		return source, "", "", false, err
	}
	if source.Volume != nil {
		return source, source.Volume.Pool, source.Volume.target(), false, nil
	}
	if source.Rootfs != nil {
		return source, source.Rootfs.Pool, source.Rootfs.target(), true, nil
	}
	if source.Base != nil {
		return source, source.Base.Pool, source.Base.target(), true, nil
	}
	return source, "", "", false, core.ErrUnsupported
}
func (r *Runtime) restoreComponent(b restoreBinding) (core.SnapshotComponent, error) {
	_, pool, _, instance, err := r.restoreShape(b)
	if err != nil {
		return core.SnapshotComponent{}, err
	}
	ref := "volume/" + pool + "/" + b.target()
	if instance {
		ref = "instance/" + b.target()
	}
	raw, err := json.Marshal(b)
	if err != nil || len(raw) > 16384 {
		return core.SnapshotComponent{}, core.ErrInvalidArgument
	}
	return core.SnapshotComponent{Role: b.Source.Role, Owner: b.Owner, NativeRef: ref, Binding: string(raw), State: "planned"}, nil
}
func (r *Runtime) decodeRestore(c core.SnapshotComponent) (restoreBinding, error) {
	var b restoreBinding
	if len(c.Binding) == 0 || len(c.Binding) > 16384 || json.Unmarshal([]byte(c.Binding), &b) != nil {
		return b, core.ErrInvalidArgument
	}
	expected, err := r.restoreComponent(b)
	if err != nil {
		return b, err
	}
	expected.State = c.State
	if expected != c {
		return b, core.ErrCapabilityStale
	}
	return b, nil
}
func restoreConfigMatches(config, expected map[string]string) bool {
	for k, v := range expected {
		if config[k] != v {
			return false
		}
	}
	for k, v := range config {
		if v != "" && !strings.HasPrefix(k, "volatile.") && expected[k] != v {
			return false
		}
	}
	return true
}
func (r *Runtime) observeRestore(ctx context.Context, b restoreBinding) (bool, error) {
	_, pool, _, instance, err := r.restoreShape(b)
	if err != nil {
		return false, err
	}
	endpoint := "/1.0/storage-pools/" + pool + "/volumes/custom?project=" + r.project + "&recursion=1"
	if instance {
		endpoint = "/1.0/instances?project=" + r.project + "&recursion=1"
	}
	out, err := r.runner.Run(ctx, "incus", "query", endpoint)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return false, core.ErrRuntimeUnavailable
	}
	found := false
	if instance {
		var list []snapshotInstanceObservation
		if json.Unmarshal([]byte(out.Stdout), &list) != nil || list == nil {
			return false, core.ErrIncompatibleState
		}
		for _, v := range list {
			if v.Name != b.target() {
				continue
			}
			if found || v.Type != "container" || strings.ToUpper(v.Status) != "STOPPED" || v.Ephemeral || len(v.Profiles) != 0 || !restoreConfigMatches(v.Config, b.config(true)) || !restoreConfigMatches(v.ExpandedConfig, b.config(true)) {
				return false, core.ErrIncompatibleState
			}
			for _, devices := range []map[string]map[string]string{v.Devices, v.ExpandedDevices} {
				roots := 0
				for _, d := range devices {
					if d["type"] == "disk" && d["path"] == "/" {
						if !reflect.DeepEqual(d, map[string]string{"type": "disk", "path": "/", "pool": pool}) {
							return false, core.ErrIncompatibleState
						}
						roots++
					} else if !reflect.DeepEqual(d, map[string]string{"type": "none"}) {
						return false, core.ErrIncompatibleState
					}
				}
				if roots != 1 {
					return false, core.ErrIncompatibleState
				}
			}
			found = true
		}
	} else {
		var list []persistentVolumeObservation
		if json.Unmarshal([]byte(out.Stdout), &list) != nil || list == nil {
			return false, core.ErrIncompatibleState
		}
		for _, v := range list {
			if v.Name != b.target() {
				continue
			}
			if found || v.Type != "custom" || v.ContentType != "filesystem" || len(v.UsedBy) != 0 || !restoreConfigMatches(v.Config, b.config(false)) {
				return false, core.ErrIncompatibleState
			}
			found = true
		}
	}
	return found, nil
}
func (r *Runtime) VerifyRestoreComponent(ctx context.Context, c core.SnapshotComponent) error {
	b, err := r.decodeRestore(c)
	if err != nil {
		return err
	}
	if c.State != "created" && c.State != "verified" {
		return core.ErrIncompatibleState
	}
	found, err := r.observeRestore(ctx, b)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrNotFound
	}
	return nil
}
func (r *Runtime) DeleteRestoreComponent(ctx context.Context, c core.SnapshotComponent) error {
	b, err := r.decodeRestore(c)
	if err != nil {
		return err
	}
	found, err := r.observeRestore(ctx, b)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	_, pool, _, instance, err := r.restoreShape(b)
	if err != nil {
		return err
	}
	args := []string{"storage", "volume", "delete", pool, b.target(), "--project", r.project}
	if instance {
		args = []string{"delete", b.target(), "--project", r.project}
	}
	out, err := r.runner.Run(ctx, "incus", args...)
	if err != nil || out.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	found, err = r.observeRestore(ctx, b)
	if err != nil {
		return err
	}
	if found {
		return core.ErrRecoveryRequired
	}
	return nil
}
