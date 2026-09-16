package incus

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ConfigureMaintenanceTooling is installed by trusted composition before serving
// requests. The OCI integration supplies files, not a guest-selected executable.
func (r *Runtime) ConfigureMaintenanceTooling(prepare func(context.Context) (string, func() error, error)) {
	r.maintenanceTooling = prepare
}

func (p *SandboxProvider) verifyMaintenanceToolTarget(ctx context.Context, ref, generation string) error {
	if validateManagedInstanceRef(ref) != nil || ref == trustedHostName || !core.ValidEnvironmentInstanceID(generation) {
		return core.ErrInvalidArgument
	}
	out, err := p.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+p.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
		return core.ErrRuntimeUnavailable
	}
	var i snapshotInstanceObservation
	if json.Unmarshal([]byte(out.Stdout), &i) != nil || i.Name != ref || i.Type != "container" || i.Status != "Running" || i.Profiles == nil || len(i.Profiles) != 0 || i.Config[environmentInstanceKey] != generation || i.ExpandedConfig[environmentInstanceKey] != generation || i.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue || (i.ExpandedConfig["security.privileged"] != "" && i.ExpandedConfig["security.privileged"] != "false") {
		return core.ErrIncompatibleState
	}
	for _, devices := range []map[string]map[string]string{i.Devices, i.ExpandedDevices} {
		if devices == nil {
			return core.ErrIncompatibleState
		}
		for name, device := range devices {
			if name == "persistent-resource" || (device["type"] == "disk" && device["path"] != "/" && device["path"] != "/workspace") {
				return core.ErrIncompatibleState
			}
		}
	}
	return nil
}

// Called only under canonical creation ownership, after preparation and before
// any retained mount. The getter does not execute downloaded tools on the Host.
func (p *SandboxProvider) provisionMaintenanceTooling(ctx context.Context, ref, generation string) (err error) {
	if p.maintenanceTooling == nil {
		return nil
	} // Explicitly pre-provisioned adapters remain version-checked at startup.
	if err = p.verifyMaintenanceToolTarget(ctx, ref, generation); err != nil {
		return err
	}
	directory, release, err := p.maintenanceTooling(ctx)
	if err != nil {
		return err
	}
	if release == nil {
		return core.ErrIncompatibleState
	}
	defer func() { err = errors.Join(err, release()) }()
	run := func(args ...string) error {
		out, e := p.runner.Run(ctx, "incus", args...)
		if e != nil {
			return e
		}
		if out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
			return core.ErrRuntimeUnavailable
		}
		return nil
	}
	if err = run("exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", maintenanceToolingPrepare); err != nil {
		return err
	}
	for _, name := range []string{"containerd", "ctr", "nerdctl"} {
		source, digest, e := trustedClientSource(filepath.Join(directory, name))
		if e != nil {
			return e
		}
		if e = p.verifyMaintenanceToolTarget(ctx, ref, generation); e != nil {
			return e
		}
		destination := "/run/hacocoon-maintenance-tools/" + name
		if e = run("file", "push", source, ref+destination, "--project", p.project, "--uid", "0", "--gid", "0", "--mode", "0755"); e != nil {
			return e
		}
		out, e := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "sha256sum", "--", destination)
		fields := strings.Fields(out.Stdout)
		if e != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated || len(fields) != 2 || fields[0] != digest || fields[1] != destination {
			return core.ErrIncompatibleState
		}
	}
	if err = p.verifyMaintenanceToolTarget(ctx, ref, generation); err != nil {
		return err
	}
	return run("exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", maintenanceToolingInstall)
}

const maintenanceToolingPrepare = `set -eu
for path in /run /usr /usr/local /usr/local/bin; do test ! -L "$path"; done
if grep -Fq ' /var/lib/hacocoon-oci ' /proc/self/mountinfo; then exit 40; else test "$?" -eq 1; fi
mkdir -p /usr/local/bin
mkdir -m 0700 /run/hacocoon-maintenance-tools
`
const maintenanceToolingInstall = `set -eu
for path in /usr /usr/local /usr/local/bin /run /run/hacocoon-maintenance-tools; do test -d "$path"; test ! -L "$path"; done
for name in containerd ctr nerdctl; do
 test -f "/run/hacocoon-maintenance-tools/$name"
 test ! -L "/run/hacocoon-maintenance-tools/$name"
 mv -T "/run/hacocoon-maintenance-tools/$name" "/usr/local/bin/$name"
done
rmdir /run/hacocoon-maintenance-tools
`
