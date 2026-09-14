package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const environmentDataKey = "user.hacocoon.data-binding"
const environmentDataDevicePrefix = "haco-data-"

func environmentDataUsedBy(raw, project, ref string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Fragment != "" || u.RawPath != "" || u.Path != "/1.0/instances/"+ref {
		return false
	}
	values, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(values) == 1 && len(values["project"]) == 1 && values.Get("project") == project
}

// Rootfs and managed-Workspace paths share syntax/protected-path checks. Their
// backing data must still be inspected through the corresponding native API.
func validEnvironmentDataTarget(target string) bool {
	if target == "" || !utf8.ValidString(target) || path.Clean(target) != target || len(target) > 1024 || strings.Contains(target, "\\") || strings.ContainsFunc(target, unicode.IsControl) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(target, "/"), "/")
	if !strings.HasPrefix(target, "/") || len(parts) < 2 {
		return false
	}
	switch parts[0] {
	case "root":
	case "workspace":
	case "home":
		if len(parts) < 3 {
			return false
		}
	case "var":
		if len(parts) < 3 || parts[1] != "cache" {
			return false
		}
	default:
		return false
	}
	for _, part := range parts {
		switch part {
		case ".ssh", ".aws", ".azure", ".kube", ".gnupg", ".docker", ".config", ".git", ".netrc", ".git-credentials", ".npmrc", ".pypirc":
			return false
		}
	}
	return true
}

// The digest binds only immutable placement/ownership, never mutable catalog
// state or credentials. Native custom volumes additionally carry the parent ID.
func environmentDataBinding(instance string, areas []core.EnvironmentRuntimeAttachment) (string, error) {
	if len(areas) > core.MaxEnvironmentAttachments {
		return "", core.ErrInvalidArgument
	}
	if len(areas) == 0 {
		return "", nil
	}
	if !core.ValidEnvironmentInstanceID(instance) {
		return "", core.ErrInvalidArgument
	}
	type entry struct {
		Attachment      core.EnvironmentAttachment
		NativeRef, Kind string
	}
	entries := make([]entry, 0, len(areas))
	attachments := make([]core.EnvironmentAttachment, 0, len(areas))
	for _, area := range areas {
		a, r := area.Attachment, area.Resource
		if !validEnvironmentDataTarget(a.Target) {
			return "", fmt.Errorf("cache placement %q is not a supported data directory: %w", a.Target, core.ErrUnsupported)
		}
		if a.Resource != r.Ref() || r.Kind != CacheResourceKind || a.Origin.Kind != r.Kind || r.State != "ready" || r.SourceOnly || r.EnvironmentInstance != instance || r.CopySource != (core.PersistentResourceRef{}) || r.CopyCompleted {
			return "", core.ErrIncompatibleState
		}
		if _, _, err := managedResourceVolume(r); err != nil {
			return "", err
		}
		attachments = append(attachments, a)
		entries = append(entries, entry{a, r.NativeRef, r.Kind})
	}
	if !core.ValidEnvironmentAttachments(attachments) {
		return "", core.ErrInvalidArgument
	}
	data, err := json.Marshal(struct {
		Instance string
		Areas    []entry
	}{instance, entries})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

type environmentDataConfiguration struct {
	Config          map[string]string            `json:"config"`
	Devices         map[string]map[string]string `json:"devices"`
	ExplicitDevices map[string]map[string]string
}

func (r *Runtime) environmentDataConfiguration(ctx context.Context, ref string) (environmentDataConfiguration, error) {
	var config environmentDataConfiguration
	if err := validateManagedInstanceRef(ref); err != nil {
		return config, err
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+r.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return config, core.ErrRuntimeUnavailable
	}
	var observed struct {
		Config          map[string]string            `json:"config"`
		Devices         map[string]map[string]string `json:"devices"`
		ExpandedConfig  map[string]string            `json:"expanded_config"`
		ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	}
	if json.Unmarshal([]byte(out.Stdout), &observed) != nil || observed.Config == nil || observed.Devices == nil || observed.ExpandedConfig == nil || observed.ExpandedDevices == nil {
		return config, core.ErrIncompatibleState
	}
	for _, key := range []string{environmentInstanceKey, environmentDataKey} {
		if observed.Config[key] != observed.ExpandedConfig[key] {
			return config, core.ErrIncompatibleState
		}
	}
	for name, device := range observed.ExpandedDevices {
		if strings.HasPrefix(name, environmentDataDevicePrefix) && !maps.Equal(device, observed.Devices[name]) {
			return config, core.ErrIncompatibleState
		}
	}
	for name, device := range observed.Devices {
		if strings.HasPrefix(name, environmentDataDevicePrefix) && !maps.Equal(device, observed.ExpandedDevices[name]) {
			return config, core.ErrIncompatibleState
		}
	}
	config.Config, config.Devices = observed.ExpandedConfig, observed.ExpandedDevices
	config.ExplicitDevices = observed.Devices
	return config, nil
}

func environmentDataDevice(area core.EnvironmentRuntimeAttachment) map[string]string {
	pool, volume, _ := managedResourceVolume(area.Resource)
	return map[string]string{"type": "disk", "pool": pool, "source": volume, "path": area.Attachment.Target}
}

func verifyEnvironmentDataDevices(config environmentDataConfiguration, instance, binding string, areas []core.EnvironmentRuntimeAttachment, mounts []WorkspaceAttachment, attached bool) error {
	if config.Config[environmentDataKey] != binding {
		return core.ErrCapabilityStale
	}
	if len(areas) != 0 && config.Config[environmentInstanceKey] != instance {
		return core.ErrCapabilityStale
	}
	for _, m := range mounts {
		if !matchesEnvironmentWorkspaceDevice(config, m.Device, config.Devices[m.Device], mounts) {
			return core.ErrIncompatibleState
		}
	}
	expected := map[string]map[string]string{}
	for _, area := range areas {
		expected[environmentDataDevicePrefix+area.Attachment.Key] = environmentDataDevice(area)
	}
	for name, device := range config.Devices {
		if strings.HasPrefix(name, environmentDataDevicePrefix) {
			want, present := expected[name]
			if !attached || !present || !maps.Equal(device, want) {
				return core.ErrIncompatibleState
			}
			delete(expected, name)
			continue
		}
		if device["type"] != "disk" || device["path"] == "/" {
			continue
		}
		for _, area := range areas {
			target, mount := area.Attachment.Target, device["path"]
			if strings.HasPrefix(target, mount+"/") && matchesEnvironmentWorkspaceDevice(config, name, device, mounts) {
				continue
			}
			if mount == "" || path.Clean(mount) != mount || !strings.HasPrefix(mount, "/") || mount == target || strings.HasPrefix(target, mount+"/") || strings.HasPrefix(mount, target+"/") {
				return core.ErrUnsupported
			}
		}
	}
	if attached && len(expected) != 0 {
		return core.ErrIncompatibleState
	}
	return nil
}

func (p *SandboxProvider) attachEnvironmentResources(ctx context.Context, ref string, request core.EnvironmentResourceBinding) error {
	instance, areas := request.InstanceID, request.Attachments
	if len(areas) == 0 {
		return nil
	}
	binding, mounts, err := p.environmentPlacementBinding(ctx, request)
	if err != nil {
		return err
	}
	config, err := p.environmentDataConfiguration(ctx, ref)
	if err != nil {
		return err
	}
	if err := verifyEnvironmentDataDevices(config, instance, binding, areas, mounts, false); err != nil {
		return err
	}
	status, err := p.InspectEnvironment(ctx, ref)
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentStopped {
		return core.ErrStorageBusy
	}
	// Verify every area before the first device mutation. Canonical lifecycle
	// owns all volumes and any partially configured runtime if a later call fails.
	backend := &PersistentResourceBackend{Runtime: p.Runtime}
	for _, area := range areas {
		if err := backend.Verify(ctx, area.Resource); err != nil {
			return err
		}
	}
	if err := p.verifyEnvironmentWorkspaceData(ctx, ref, mounts); err != nil {
		return err
	}
	if err := p.verifyEnvironmentDataPaths(ctx, ref, areas, mounts); err != nil {
		return err
	}
	for _, area := range areas {
		d := environmentDataDevice(area)
		out, err := p.runner.Run(ctx, "incus", "config", "device", "add", ref, environmentDataDevicePrefix+area.Attachment.Key, "disk", "pool="+d["pool"], "source="+d["source"], "path="+d["path"], "--project", p.project)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			return core.ErrRecoveryRequired
		}
	}
	return p.verifyEnvironmentResources(ctx, ref, request, core.EnvironmentStopped)
}

func (p *SandboxProvider) verifyEnvironmentResources(ctx context.Context, ref string, request core.EnvironmentResourceBinding, state core.EnvironmentState) error {
	instance, areas := request.InstanceID, request.Attachments
	binding, mounts, err := p.environmentPlacementBinding(ctx, request)
	if err != nil {
		return err
	}
	config, err := p.environmentDataConfiguration(ctx, ref)
	if err != nil {
		return err
	}
	if err := verifyEnvironmentDataDevices(config, instance, binding, areas, mounts, true); err != nil {
		return err
	}
	backend := &PersistentResourceBackend{Runtime: p.Runtime}
	for _, area := range areas {
		volume, err := backend.observe(ctx, area.Resource)
		if err != nil {
			return err
		}
		if volume == nil {
			return core.ErrNotFound
		}
		if len(volume.UsedBy) != 1 || !environmentDataUsedBy(volume.UsedBy[0], p.project, ref) {
			return core.ErrStorageBusy
		}
	}
	if err := p.verifyEnvironmentWorkspaceData(ctx, ref, mounts); err != nil {
		return err
	}
	if len(areas) != 0 && state == core.EnvironmentStopped {
		return p.verifyEnvironmentDataPaths(ctx, ref, areas, mounts)
	}
	return nil
}
