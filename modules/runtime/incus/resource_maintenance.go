package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Preparation runs only in a newly owned disposable Environment, before the
// retained Store is attached. It never runs in haco-host or on the Physical Host.
// Daemon-specific safe startup and Store attachment are separate subsequent steps.
const resourceMaintenancePreparation = `set -eu
# Refuse preparation after data attachment, even if a caller reverses the order.
if grep -Fq ' /var/lib/hacocoon-oci ' /proc/self/mountinfo; then
  exit 40
else
  test "$?" -eq 1
fi
for unit in docker.socket docker.service hacocoon-docker.socket hacocoon-docker.service containerd.service buildkit.service; do
  state=$(systemctl show --property=LoadState --value "$unit")
  case "$state" in
    loaded|masked) systemctl stop "$unit" ;;
    not-found) ;;
    *) exit 41 ;;
  esac
  # These files belong only to this disposable rootfs; retained data is absent.
  rm -f "/etc/systemd/system/$unit"
  ln -s /dev/null "/etc/systemd/system/$unit"
done
systemctl daemon-reload
for unit in docker.socket docker.service hacocoon-docker.socket hacocoon-docker.service containerd.service buildkit.service; do
  test "$(systemctl show --property=LoadState --value "$unit")" = masked
  case "$(systemctl show --property=ActiveState --value "$unit")" in inactive|failed) ;; *) exit 42 ;; esac
done
# Catch daemons started outside these units. Do not kill or adopt unknown tasks.
for path in /proc/[0-9]*/comm; do
  name=''
  if ! read -r name < "$path"; then continue; fi
  case "$name" in dockerd|containerd|containerd-shim*|buildkitd) exit 43 ;; esac
done
`

func (p *SandboxProvider) prepareResourceMaintenance(ctx context.Context, ref string) error {
	if validateManagedInstanceRef(ref) != nil || ref == trustedHostName {
		return core.ErrInvalidArgument
	}
	result, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "/bin/sh", "-ec", resourceMaintenancePreparation)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
