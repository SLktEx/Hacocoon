package incus

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
)

//go:embed host_tooling.py
var hostToolingScript string

// Incus start and systemctl's private socket can be ready before the system
// D-Bus used by systemd-run. Probe that exact bus before dispatching a mutation.
const hostToolingSystemdReady = `attempt=0
until /usr/bin/busctl --system --timeout=1s get-property org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager Version >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  test "$attempt" -lt 60 || exit 1
  sleep 0.5
done
exec "$@"
`

// ProvisionHostTools belongs to the maintained local integration, not Core or
// Environment provisioning. The same lock excludes Host start and area copying.
func (b *PersistentResourceBackend) ProvisionHostTools(ctx context.Context, source core.PersistentResource) error {
	if source.State != "ready" || source.WorkspaceID != "" {
		return core.ErrRecoveryRequired
	}
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return err
	}
	defer unlock()
	if err := b.VerifyHostSource(ctx, source); err != nil {
		return err
	}
	i, err := b.hostCopyInstance(ctx, source)
	if err != nil {
		return err
	}
	if i.Type != "container" || i.Profiles == nil || len(i.Profiles) != 0 ||
		i.LocalConfig[trustedHostRoleKey] != trustedHostRoleValue ||
		i.LocalConfig[hostOCIStoreKey] != source.Owner ||
		i.LocalConfig["security.nesting"] != "true" || i.Config["security.nesting"] != "true" ||
		(i.Config["security.privileged"] != "" && i.Config["security.privileged"] != "false") {
		return core.ErrRecoveryRequired
	}
	for _, stage := range []string{"host_packages", "host_tooling", "host_services"} {
		if err := hostsetup.Step(ctx, stage, func() error {
			seconds := int64(600)
			if deadline, ok := ctx.Deadline(); ok {
				if remaining := int64(time.Until(deadline).Seconds()) - 10; remaining < seconds {
					seconds = remaining
				}
			}
			if seconds < 1 {
				return context.DeadlineExceeded
			}
			// Guest cgroup timeout and fixed unit also bound descendants/overlap
			// after controller loss. No helper output is copied into diagnostics.
			result, runErr := b.Runtime.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--",
				"/bin/sh", "-ec", hostToolingSystemdReady, "haco-host-tooling",
				"/usr/bin/systemd-run", "--unit=hacocoon-host-tooling", "--description=Hacocoon standard Host tooling", "--expand-environment=no", "--collect", "--wait", "--pipe", "--quiet",
				"--service-type=exec", "--working-directory=/root", "--property=KillMode=control-group",
				"--property=TimeoutStopSec=5s", fmt.Sprintf("--property=RuntimeMaxSec=%ds", seconds),
				"/usr/bin/python3", "-I", "-c", hostToolingScript, stage)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if runErr != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
				return core.ErrRuntimeUnavailable
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return b.VerifyHostSource(ctx, source)
}
