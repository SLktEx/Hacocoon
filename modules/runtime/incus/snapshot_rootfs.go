package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type snapshotRootfsPlan struct{ Pool, Source, SourceInstanceID, Owner string }

func (p snapshotRootfsPlan) target() string { return "haco-snapshot-root-" + p.Owner }
func (p snapshotRootfsPlan) validate() error {
	if !safeIncusRef(p.Pool) || p.Source == trustedHostName || p.Source == p.target() || !core.ValidEnvironmentInstanceID(p.SourceInstanceID) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: p.Owner}) {
		return core.ErrInvalidArgument
	}
	return validateManagedInstanceRef(p.Source)
}
func (p snapshotRootfsPlan) config() map[string]string {
	return map[string]string{
		"user.hacocoon.kind": "snapshot-rootfs", "user.hacocoon.owner": p.Owner, "user.hacocoon.snapshot-source": p.Source,
		"user.hacocoon.snapshot-instance": p.SourceInstanceID, "boot.autostart": "false", "security.privileged": "false", "security.nesting": "false",
	}
}

type snapshotInstanceObservation struct {
	Name            string                       `json:"name"`
	Type            string                       `json:"type"`
	Status          string                       `json:"status"`
	Ephemeral       bool                         `json:"ephemeral"`
	Profiles        []string                     `json:"profiles"`
	Config          map[string]string            `json:"config"`
	ExpandedConfig  map[string]string            `json:"expanded_config"`
	Devices         map[string]map[string]string `json:"devices"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
}

func (r *Runtime) snapshotRootfsObservation(ctx context.Context, p snapshotRootfsPlan, target bool) (*snapshotInstanceObservation, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	ref := p.Source
	if target {
		ref = p.target()
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
	for _, i := range instances {
		if i.Name != ref {
			continue
		}
		if found != nil || i.Type != "container" || strings.ToUpper(i.Status) != "STOPPED" || i.Config == nil || i.ExpandedConfig == nil || i.Devices == nil || i.ExpandedDevices == nil {
			return nil, core.ErrIncompatibleState
		}
		expected := map[string]string{environmentInstanceKey: p.SourceInstanceID, managedEnvironmentMarkerKey: managedEnvironmentMarkerValue}
		if target {
			expected = p.config()
			if i.Ephemeral || len(i.Profiles) != 0 {
				return nil, core.ErrIncompatibleState
			}
		}
		for key, value := range expected {
			if i.Config[key] != value || i.ExpandedConfig[key] != value {
				return nil, core.ErrCapabilityStale
			}
		}
		roots := 0
		for _, device := range i.ExpandedDevices {
			if device["type"] == "disk" && device["path"] == "/" {
				if device["pool"] != p.Pool || device["source"] != "" || (target && len(device) != 3) {
					return nil, core.ErrIncompatibleState
				}
				roots++
				continue
			}
			if target && (len(device) != 1 || device["type"] != "none") {
				return nil, core.ErrIncompatibleState
			}
		}
		if roots != 1 {
			return nil, core.ErrIncompatibleState
		}
		if target {
			for key, value := range i.ExpandedConfig {
				if value == "" || strings.HasPrefix(key, "volatile.") {
					continue
				}
				if expected[key] != value {
					return nil, core.ErrIncompatibleState
				}
			}
		}
		copy := i
		found = &copy
	}
	return found, nil
}

// createSnapshotRootfs copies only the root disk into an independent stopped
// instance, with authority-bearing config cleared and other devices masked.
// The complete binding must already be durably reserved by the coordinator.
func (r *Runtime) createSnapshotRootfs(ctx context.Context, p snapshotRootfsPlan) error {
	source, err := r.snapshotRootfsObservation(ctx, p, false)
	if err != nil {
		return err
	}
	if source == nil {
		return core.ErrNotFound
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var pool struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &pool) != nil || pool.Name != p.Pool || pool.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	// Incus fills omitted keys from the source. Explicit empty values prevent
	// inheritance, without sending arbitrary source config values to the command.
	config := map[string]string{}
	for key := range source.Config {
		config[key] = ""
	}
	for key := range source.ExpandedConfig {
		config[key] = ""
	}
	for _, key := range []string{"volatile.idmap.current", "volatile.idmap.next", "volatile.last_state.idmap"} {
		value := source.Config[key]
		if value == "" {
			continue
		}
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return core.ErrIncompatibleState
		}
		config[key] = value
	}
	for key, value := range p.config() {
		config[key] = value
	}
	devices := map[string]map[string]string{}
	for name := range source.Devices {
		devices[name] = map[string]string{"type": "none"}
	}
	for name, device := range source.ExpandedDevices {
		devices[name] = map[string]string{"type": "none"}
		if device["type"] == "disk" && device["path"] == "/" {
			devices[name] = map[string]string{"type": "disk", "path": "/", "pool": p.Pool}
		}
	}
	data, err := json.Marshal(map[string]any{"name": p.target(), "type": "container", "ephemeral": false, "profiles": []string{}, "config": config, "devices": devices,
		"source": map[string]any{"type": "copy", "source": p.Source, "project": r.project, "instance_only": true, "live": false}})
	if err != nil {
		return err
	}
	out, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/instances?project="+r.project, "--data", string(data))
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return fmt.Errorf("snapshot rootfs creation unconfirmed: %w", core.ErrRecoveryRequired)
	}
	// No provider call after create: the caller must first record the receipt.
	return nil
}
func (r *Runtime) verifySnapshotRootfs(ctx context.Context, p snapshotRootfsPlan) error {
	i, err := r.snapshotRootfsObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if i == nil {
		return core.ErrNotFound
	}
	return nil
}
func (r *Runtime) deleteSnapshotRootfs(ctx context.Context, p snapshotRootfsPlan) error {
	i, err := r.snapshotRootfsObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if i == nil {
		return nil
	}
	out, err := r.runner.Run(ctx, "incus", "delete", p.target(), "--project", r.project)
	if err != nil || out.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	i, err = r.snapshotRootfsObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if i != nil {
		return core.ErrRecoveryRequired
	}
	return nil
}
