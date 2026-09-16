package incus

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// startContainerdMaintenance starts metadata services only. It does not expose a
// general execution API or enable the unfinished public maintenance lifecycle.
// The caller holds the canonical Environment/Store lease until runtime absence.
func (p *SandboxProvider) startContainerdMaintenance(ctx context.Context, ref, generation string, resource core.PersistentResource) error {
	if validateManagedInstanceRef(ref) != nil || ref == trustedHostName || !core.ValidEnvironmentInstanceID(generation) || resource.SourceOnly || resource.State != "ready" || resource.Kind != OCIStoreKind {
		return core.ErrInvalidArgument
	}
	pool, volume, err := persistentVolume(resource)
	if err != nil {
		return err
	}
	observed, err := (&PersistentResourceBackend{Runtime: p.Runtime}).observe(ctx, resource)
	if err != nil {
		return err
	}
	if observed == nil || len(observed.UsedBy) != 1 {
		return core.ErrStorageBusy
	}
	u, err := url.Parse(observed.UsedBy[0])
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" || u.RawPath != "" || u.Path != "/1.0/instances/"+ref {
		return core.ErrIncompatibleState
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) != 1 || len(q["project"]) != 1 || q.Get("project") != p.project {
		return core.ErrIncompatibleState
	}
	out, err := p.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+p.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var instance snapshotInstanceObservation
	if json.Unmarshal([]byte(out.Stdout), &instance) != nil || instance.Name != ref || instance.Type != "container" || instance.Status != "Running" || instance.Profiles == nil || len(instance.Profiles) != 0 || instance.Config[environmentInstanceKey] != generation || instance.ExpandedConfig[environmentInstanceKey] != generation || instance.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue || (instance.ExpandedConfig["security.privileged"] != "" && instance.ExpandedConfig["security.privileged"] != "false") {
		return core.ErrIncompatibleState
	}
	expected := map[string]string{"type": "disk", "pool": pool, "source": volume, "path": OCIStorePath}
	if !reflect.DeepEqual(instance.Devices["persistent-resource"], expected) || !reflect.DeepEqual(instance.ExpandedDevices["persistent-resource"], expected) {
		return core.ErrIncompatibleState
	}
	for name, device := range instance.ExpandedDevices {
		if name != "persistent-resource" && (device["path"] == OCIStorePath || strings.HasPrefix(device["path"], OCIStorePath+"/")) {
			return core.ErrIncompatibleState
		}
	}
	out, err = p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "/bin/sh", "-ec", containerdMaintenanceStart)
	if out.ExitCode == 42 && !out.StdoutTruncated && !out.StderrTruncated {
		return core.ErrUnsupported
	}
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
		return core.ErrRuntimeUnavailable
	}
	return nil
}

// Configuration is generated in a fresh private guest /run directory, never
// loaded from retained data. Keep version acceptance explicit until other native
// versions are tested. No restart labels, container records or policies are edited.
const containerdMaintenanceStart = `set -eu
for unit in docker.socket docker.service hacocoon-docker.socket hacocoon-docker.service containerd.service buildkit.service; do
 test "$(systemctl show --property=LoadState --value "$unit")" = masked
 case "$(systemctl show --property=ActiveState --value "$unit")" in inactive|failed) ;; *) exit 40 ;; esac
done
for path in /proc/[0-9]*/comm; do
 name=''
 if ! read -r name < "$path"; then continue; fi
 case "$name" in dockerd|containerd|containerd-shim*|buildkitd) exit 41 ;; esac
done
case "$(/usr/local/bin/containerd --version)" in 'containerd github.com/containerd/containerd/v2 v2.3.3 '*) ;; *) exit 42 ;; esac
for path in /run /var/lib/hacocoon-oci /var/lib/hacocoon-oci/containerd; do
 test -d "$path" && test ! -L "$path"
done
grep -Fq ' /var/lib/hacocoon-oci ' /proc/self/mountinfo
umask 077
mkdir /run/hacocoon-maintenance
mkdir /run/hacocoon-maintenance/state
cat > /run/hacocoon-maintenance/containerd.toml <<'HACO'
version = 3
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/hacocoon-maintenance/state"
disabled_plugins = ["io.containerd.monitor.container.v1.restart", "io.containerd.grpc.v1.cri", "io.containerd.cri.v1.images", "io.containerd.cri.v1.runtime", "io.containerd.nri.v1.nri", "io.containerd.runtime.v2.task", "io.containerd.service.v1.tasks-service", "io.containerd.grpc.v1.tasks", "io.containerd.sandbox.controller.v1.podsandbox", "io.containerd.sandbox.controller.v1.shim"]
[grpc]
  address = "/run/hacocoon-maintenance/containerd.sock"
HACO
systemd-run --unit=hacocoon-maintenance-containerd --property=Restart=no --property=KillMode=control-group --property=UMask=0077 /usr/local/bin/containerd --config /run/hacocoon-maintenance/containerd.toml
n=0
until /usr/local/bin/ctr --address /run/hacocoon-maintenance/containerd.sock version >/dev/null 2>&1; do
 n=$((n+1)); test "$n" -lt 60
 systemctl is-active --quiet hacocoon-maintenance-containerd
 sleep 0.5
done
`
