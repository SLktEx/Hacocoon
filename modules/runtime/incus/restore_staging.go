package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// restoreBinding is an independent staging copy, not a runnable Environment.
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
func (r *Runtime) PlanSnapshotRestore(ctx context.Context, saved core.Snapshot, id string) ([]core.SnapshotComponent, error) {
	if !strings.HasPrefix(id, "restore-") || len(id) != 40 || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:restore", Owner: strings.TrimPrefix(id, "restore-")}) || saved.State != "ready" || len(saved.Components) < 2 || len(saved.Components) > 256 {
		return nil, core.ErrInvalidArgument
	}
	out := []core.SnapshotComponent{}
	for _, src := range saved.Components {
		// Legacy Base material remains catalogued, but rootfs is self-contained.
		if src.Role == "base" {
			continue
		}
		if err := r.VerifySnapshotComponent(ctx, src); err != nil {
			return nil, err
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		c, err := r.restoreComponent(restoreBinding{Version: 1, Project: r.project, Owner: hex.EncodeToString(nonce[:]), Source: src})
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
func (r *Runtime) CreateRestoreComponent(ctx context.Context, saved core.Snapshot, c core.SnapshotComponent) error {
	b, err := r.decodeRestore(c)
	if err != nil {
		return err
	}
	if c.State != "planned" || saved.State != "ready" {
		return core.ErrIncompatibleState
	}
	matches := 0
	for _, src := range saved.Components {
		if src == b.Source {
			matches++
		}
	}
	if matches != 1 {
		return core.ErrCapabilityStale
	}
	source, pool, native, instance, err := r.restoreShape(b)
	if err != nil {
		return err
	}
	var sourceConfig map[string]string
	sourceDevices := map[string]map[string]string{}
	if instance {
		var found *snapshotInstanceObservation
		if source.Rootfs != nil {
			found, err = r.snapshotRootfsObservation(ctx, *source.Rootfs, true)
		} else {
			found, err = r.snapshotBaseObservation(ctx, *source.Base)
		}
		if err != nil {
			return err
		}
		if found == nil {
			return core.ErrNotFound
		}
		sourceConfig = found.Config
		for k := range found.Devices {
			sourceDevices[k] = map[string]string{"type": "none"}
		}
		for k := range found.ExpandedDevices {
			sourceDevices[k] = map[string]string{"type": "none"}
		}
	} else {
		found, err := r.snapshotVolumeObservation(ctx, *source.Volume, true)
		if err != nil {
			return err
		}
		if found == nil {
			return core.ErrNotFound
		}
		sourceConfig = found.Config
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var storage struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &storage) != nil || storage.Name != pool || storage.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	config := map[string]string{}
	// Explicit empty values prevent inheritance of source ownership/config.
	for k := range sourceConfig {
		config[k] = ""
	}
	keys := []string{"volatile.idmap.last", "volatile.idmap.next"}
	if instance {
		keys = []string{"volatile.idmap.current", "volatile.idmap.next", "volatile.last_state.idmap"}
	}
	for _, k := range keys {
		value := sourceConfig[k]
		if value == "" {
			continue
		}
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return core.ErrIncompatibleState
		}
		config[k] = value
	}
	for k, v := range b.config(instance) {
		config[k] = v
	}
	endpoint := "/1.0/storage-pools/" + pool + "/volumes/custom?project=" + r.project
	request := map[string]any{"name": b.target(), "type": "custom", "content_type": "filesystem", "config": config, "source": map[string]any{"type": "copy", "name": native, "pool": pool, "project": r.project, "volume_only": true}}
	if instance {
		sourceDevices["root"] = map[string]string{"type": "disk", "path": "/", "pool": pool}
		endpoint = "/1.0/instances?project=" + r.project
		request = map[string]any{"name": b.target(), "type": "container", "ephemeral": false, "profiles": []string{}, "config": config, "devices": sourceDevices, "source": map[string]any{"type": "copy", "source": native, "project": r.project, "instance_only": true, "live": false}}
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	out, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", endpoint, "--data", string(data))
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	return nil // Caller durably records completion before another provider call.
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
