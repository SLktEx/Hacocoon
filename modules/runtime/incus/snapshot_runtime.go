package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// CreateEnvironmentFromSnapshot copies only saved rootfs. The caller reserves
// the saved aggregate and canonical creation lease, supplies new data bindings,
// records the receipt immediately, and owns failure cleanup/publication.
func (p *SandboxProvider) CreateEnvironmentFromSnapshot(ctx context.Context, spec core.EnvironmentRuntimeSpec, saved core.Snapshot, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil || record == nil || !core.ValidEnvironmentInstanceID(spec.InstanceID) || spec.WorkspacePath == "" || spec.TemporaryWorkspace || spec.Base != "" || saved.State != "ready" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	ref := "haco-" + spec.Name
	if err := validateManagedInstanceRef(ref); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	var root *snapshotRootfsPlan
	for _, c := range saved.Components {
		if c.Role != "rootfs" {
			continue
		}
		binding, err := p.decodeSavedComponent(c)
		if err != nil {
			return core.EnvironmentRuntime{}, err
		}
		if root != nil || binding.Rootfs == nil || c.State != "verified" {
			return core.EnvironmentRuntime{}, core.ErrIncompatibleState
		}
		root = binding.Rootfs
	}
	if root == nil || root.SourceInstanceID == spec.InstanceID {
		return core.EnvironmentRuntime{}, core.ErrCapabilityStale
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := p.ensureProject(ctx); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := p.ensureRoutedSandboxHost(ctx); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	config, err := p.sandboxProfileConfig(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	config[environmentInstanceKey] = spec.InstanceID
	config[managedEnvironmentMarkerKey] = managedEnvironmentMarkerValue
	config["boot.autostart"] = "false"
	config["security.privileged"] = "false"
	config["security.nesting"] = "false"
	masks, err := p.copySavedRuntime(ctx, *root, ref, config)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	created := core.EnvironmentRuntime{Ref: ref, Resources: resources}
	if saved.Source.Environment.Base != nil {
		provenance := *saved.Source.Environment.Base
		created.Base = &provenance
	}
	// No provider call between native creation and durable ownership.
	if err := record(created); err != nil {
		return created, err
	}
	// Incus retains explicit none devices used to mask the saved source's
	// attachments. Remove only these names on the newly owned instance before
	// the ordinary configuration path adds current attachments with those names.
	for _, name := range masks {
		out, err := p.runner.Run(ctx, "incus", "config", "device", "remove", "--project", p.project, "--", ref, name)
		if err != nil || out.ExitCode != 0 {
			return created, fmt.Errorf("remove restored device mask: %w", core.ErrRuntimeUnavailable)
		}
	}
	if err := p.configureSandboxEnvironment(ctx, ref, spec, resources, false); err != nil {
		return created, err
	}
	out, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", restoredSSHIdentity)
	if err != nil || out.ExitCode != 0 {
		return created, fmt.Errorf("renew restored Environment SSH identity: %w", core.ErrRuntimeUnavailable)
	}
	return created, nil
}

func (r *Runtime) copySavedRuntime(ctx context.Context, source snapshotRootfsPlan, target string, current map[string]string) ([]string, error) {
	if err := validateManagedInstanceRef(target); err != nil {
		return nil, err
	}
	if target == source.target() || !core.ValidEnvironmentInstanceID(current[environmentInstanceKey]) || current[environmentInstanceKey] == source.SourceInstanceID || current[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue || current["security.privileged"] != "false" || current["boot.autostart"] != "false" {
		return nil, core.ErrInvalidArgument
	}
	observed, err := r.snapshotRootfsObservation(ctx, source, true)
	if err != nil {
		return nil, err
	}
	if observed == nil {
		return nil, core.ErrNotFound
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+source.Pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var storage struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &storage) != nil || storage.Name != source.Pool || storage.Driver != "btrfs" {
		return nil, core.ErrUnsupported
	}
	config := map[string]string{}
	for k := range observed.Config {
		config[k] = ""
	}
	for _, k := range []string{"volatile.idmap.current", "volatile.idmap.next", "volatile.last_state.idmap"} {
		value := observed.Config[k]
		if value == "" {
			continue
		}
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return nil, core.ErrIncompatibleState
		}
		config[k] = value
	}
	for k, v := range current {
		config[k] = v
	}
	devices := map[string]map[string]string{}
	for k := range observed.Devices {
		devices[k] = map[string]string{"type": "none"}
	}
	for k := range observed.ExpandedDevices {
		devices[k] = map[string]string{"type": "none"}
	}
	devices["root"] = map[string]string{"type": "disk", "path": "/", "pool": source.Pool}
	request := map[string]any{"name": target, "type": "container", "ephemeral": false, "profiles": []string{}, "config": config, "devices": devices,
		"source": map[string]any{"type": "copy", "source": source.target(), "project": r.project, "instance_only": true, "live": false}}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	out, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/instances?project="+r.project, "--data", string(raw))
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRecoveryRequired
	}
	var masks []string
	for name := range devices {
		if name != "root" {
			masks = append(masks, name)
		}
	}
	sort.Strings(masks)
	return masks, nil
}

// Guest-local reset runs before publication/proxy exposure. Custom user keys
// remain, while managed authorization entries and server host identity are fresh.
const restoredSSHIdentity = `set -eu
for dir in /root /root/.ssh /etc /etc/ssh; do
 test ! -L "$dir"
 if test -e "$dir"; then test -d "$dir"; fi
done
file=/root/.ssh/authorized_keys
test ! -L "$file"
if test -e "$file"; then
 test -f "$file"
 tmp="$(mktemp /root/.ssh/authorized_keys.restore.XXXXXX)"
 trap 'rm -f -- "$tmp"' EXIT
 awk 'index($0, " haco:ssh-") == 0 { print }' "$file" > "$tmp"
 chmod 600 "$tmp"
 mv -- "$tmp" "$file"
 trap - EXIT
fi
if command -v ssh-keygen >/dev/null 2>&1; then
 for key in /etc/ssh/ssh_host_*_key /etc/ssh/ssh_host_*_key.pub; do
  test ! -L "$key"
  if test -e "$key"; then test -f "$key"; rm -f -- "$key"; fi
 done
 ssh-keygen -A
 if command -v sshd >/dev/null 2>&1; then
  attempt=0
  until systemctl show --property=Version --value >/dev/null 2>&1; do
   attempt=$((attempt + 1)); test "$attempt" -lt 60; sleep 0.5
  done
  systemctl restart ssh.service
 fi
fi
`
