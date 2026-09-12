package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const hostOCICopyKey = "user.hacocoon.oci-copy"

// Provider-side journal accompanies the catalog's durable CopySource reservation.
// Disabling autostart in the same PATCH prevents a reboot from restarting writers.
type hostOCICopyJournal struct {
	Version           int    `json:"version,omitempty"`
	ExpandedAutostart string `json:"expanded_autostart,omitempty"`
	Owner             string `json:"owner"`
	Autostart         string `json:"autostart"`
}

type hostOCICopyInstance struct {
	Type        string                       `json:"type"`
	Profiles    []string                     `json:"profiles"`
	Name        string                       `json:"name"`
	StatusCode  int                          `json:"status_code"`
	Config      map[string]string            `json:"expanded_config"`
	LocalConfig map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"expanded_devices"`
}

func (b *PersistentResourceBackend) hostCopyInstance(ctx context.Context, source core.PersistentResource) (hostOCICopyInstance, error) {
	var instance hostOCICopyInstance
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/instances/"+trustedHostName+"?project="+b.Runtime.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return instance, core.ErrRuntimeUnavailable
	}
	if json.Unmarshal([]byte(result.Stdout), &instance) != nil || instance.Name != trustedHostName || instance.LocalConfig == nil || instance.Config[trustedHostRoleKey] != trustedHostRoleValue {
		return instance, core.ErrIncompatibleState
	}
	pool, name, err := persistentVolume(source)
	if err != nil {
		return instance, err
	}
	matches := 0
	for _, device := range instance.Devices {
		if device["type"] == "disk" && device["pool"] == pool && device["source"] == name {
			if device["path"] != OCIStorePath {
				return instance, core.ErrIncompatibleState
			}
			matches++
		}
	}
	if matches != 1 {
		return instance, core.ErrIncompatibleState
	}
	return instance, nil
}

func (b *PersistentResourceBackend) hostCopyConsumer(source core.PersistentResource, observed *persistentVolumeObservation) bool {
	if source.ID != "oci-source:host" || !source.SourceOnly || observed == nil || len(observed.UsedBy) != 1 {
		return false
	}
	u, err := url.Parse(observed.UsedBy[0])
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" || u.RawPath != "" || u.Path != "/1.0/instances/"+trustedHostName {
		return false
	}
	values, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(values) == 1 && len(values["project"]) == 1 && values.Get("project") == b.Runtime.project
}

func (b *PersistentResourceBackend) patchHostCopy(ctx context.Context, config map[string]any) error {
	data, err := json.Marshal(map[string]any{"config": config})
	if err != nil {
		return err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "-X", "PATCH", "--wait", "/1.0/instances/"+trustedHostName+"?project="+b.Runtime.project, "--data", string(data))
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	return nil
}

// quiesceHostCopy is called only after the source/destination reservation exists.
// Failure intentionally leaves the journal/autostart guard in place. It never
// installs a defer that might resume writers while an Incus copy still runs.
func (b *PersistentResourceBackend) quiesceHostCopy(ctx context.Context, source, target core.PersistentResource, observed *persistentVolumeObservation) (func(context.Context) error, error) {
	if !b.hostCopyConsumer(source, observed) {
		return nil, core.ErrStorageBusy
	}
	before, err := b.hostCopyInstance(ctx, source)
	if err != nil {
		return nil, err
	}
	if before.Config[hostOCICopyKey] != "" {
		return nil, core.ErrRecoveryRequired
	}
	if before.StatusCode != 103 {
		return nil, core.ErrStorageBusy
	} // Never adopt someone else's pause.
	// Setup validation can become stale before a later Environment creation.
	if err := b.VerifyHostSource(ctx, source); err != nil {
		return nil, err
	}
	prior := before.LocalConfig["boot.autostart"]
	expandedAutostart := before.Config["boot.autostart"]
	if prior != "" && prior != "true" && prior != "false" {
		return nil, core.ErrIncompatibleState
	}
	encoded, _ := json.Marshal(hostOCICopyJournal{Version: 1, Owner: target.Owner, Autostart: prior, ExpandedAutostart: expandedAutostart})
	marker := string(encoded)
	if err := b.patchHostCopy(ctx, map[string]any{hostOCICopyKey: marker, "boot.autostart": "false"}); err != nil {
		return nil, err
	}
	verify := func(ctx context.Context, status int) error {
		instance, err := b.hostCopyInstance(ctx, source)
		if err != nil {
			return err
		}
		if instance.StatusCode != status || instance.Config[hostOCICopyKey] != marker || instance.Config["boot.autostart"] != "false" {
			return core.ErrRecoveryRequired
		}
		volume, err := b.observe(ctx, source)
		if err != nil {
			return err
		}
		if !b.hostCopyConsumer(source, volume) {
			return core.ErrRecoveryRequired
		}
		return nil
	}
	if err := verify(ctx, 103); err != nil {
		return nil, err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "pause", trustedHostName, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return nil, core.ErrRecoveryRequired
	}
	if err := verify(ctx, 110); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		// Only a positively completed copy reaches this continuation.
		if err := verify(ctx, 110); err != nil {
			return err
		}
		result, err := b.Runtime.runner.Run(ctx, "incus", "start", trustedHostName, "--project", b.Runtime.project)
		if err != nil || result.ExitCode != 0 {
			return core.ErrRecoveryRequired
		}
		if err := verify(ctx, 103); err != nil {
			return err
		}
		var restored any = prior
		if prior == "" {
			restored = nil
		}
		if err := b.patchHostCopy(ctx, map[string]any{hostOCICopyKey: nil, "boot.autostart": restored}); err != nil {
			return err
		}
		after, err := b.hostCopyInstance(ctx, source)
		if err != nil {
			return err
		}
		if after.StatusCode != 103 || after.Config[hostOCICopyKey] != "" || after.Config["boot.autostart"] != expandedAutostart || after.LocalConfig["boot.autostart"] != prior {
			return core.ErrRecoveryRequired
		}
		return nil
	}, nil
}

func (r *Runtime) rejectPendingHostCopy(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "incus", "config", "get", trustedHostName, hostOCICopyKey, "--project", r.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return fmt.Errorf("Host OCI copy requires recovery before Host entry: %w", core.ErrRecoveryRequired)
	}
	return nil
}
