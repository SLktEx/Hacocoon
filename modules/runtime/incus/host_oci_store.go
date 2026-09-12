package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const hostOCIStoreKey = "user.hacocoon.oci-store-ready"

// Fresh layout only. Existing runtime data/configuration needs an area-preserving
// migration, not an empty mount or image-by-image reconstruction.
const hostOCIEmptyLayout = `set -eu
for path in /var/lib/containerd /var/lib/docker /var/lib/buildkit /var/lib/hacocoon-oci; do
 if test -L "$path"; then exit 40; fi
 if test -e "$path"; then
  test -d "$path" || exit 40
  entries=$(find "$path" -mindepth 1 -maxdepth 1 -print -quit) || exit 40
  test -z "$entries" || exit 40
 fi
done
for path in /etc/containerd/config.toml /etc/docker/daemon.json /etc/systemd/system/buildkit.service; do
 if test -e "$path" || test -L "$path"; then exit 40; fi
done
`

// Host fresh-layout preflight is stricter than Environment configuration.
const hostOCIConfiguration = persistentOCIConfiguration

// Read only known daemon configuration; never return its contents to the controller.
const hostOCILayoutVerify = `import json, os, stat
try:
    data = []
    for path in ['/etc/containerd/config.toml', '/etc/docker/daemon.json']:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, 'rb') as f:
            if not stat.S_ISREG(os.fstat(f.fileno()).st_mode): raise ValueError()
            content = f.read(8193)
        if len(content) > 8192: raise ValueError()
        data.append(content.decode('utf-8'))
    expected = '''version = 2
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/containerd"
[grpc]
  address = "/run/containerd/containerd.sock"'''
    if data[0].strip() != expected: raise ValueError()
    docker = json.loads(data[1])
    if docker != {'data-root': '/var/lib/hacocoon-oci/docker', 'exec-root': '/run/docker'}: raise ValueError()
except Exception:
    raise SystemExit(40)
`

func (b *PersistentResourceBackend) PrepareHostSource(ctx context.Context, source core.PersistentResource) error {
	if !source.SourceOnly || source.ID != "oci-source:host" || source.State != "creating" || source.WorkspaceID != "" {
		return core.ErrInvalidArgument
	}
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return err
	}
	defer unlock()
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	if err := b.Runtime.rejectPendingHostCopy(ctx); err != nil {
		return err
	}
	if err := b.Verify(ctx, source); err != nil {
		return err
	} // still detached and owned
	result, err := b.Runtime.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--", "/bin/sh", "-ec", hostOCIEmptyLayout)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return fmt.Errorf("existing Host OCI data/configuration requires area-preserving migration: %w", core.ErrRecoveryRequired)
	}
	pool, name, err := persistentVolume(source)
	if err != nil {
		return err
	}
	result, err = b.Runtime.runner.Run(ctx, "incus", "config", "device", "add", trustedHostName, "haco-oci-store", "disk", "pool="+pool, "source="+name, "path="+OCIStorePath, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	result, err = b.Runtime.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--", "/bin/sh", "-ec", hostOCIConfiguration)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	result, err = b.Runtime.runner.Run(ctx, "incus", "config", "set", trustedHostName, hostOCIStoreKey, source.Owner, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	return b.VerifyHostSource(ctx, source)
}

func (b *PersistentResourceBackend) VerifyHostSource(ctx context.Context, source core.PersistentResource) error {
	observed, err := b.observe(ctx, source)
	if err != nil {
		return err
	}
	if !b.hostCopyConsumer(source, observed) {
		return core.ErrRecoveryRequired
	}
	instance, err := b.hostCopyInstance(ctx, source)
	if err != nil {
		return err
	}
	if instance.Config[hostOCIStoreKey] != source.Owner || strings.TrimSpace(instance.Config[hostOCICopyKey]) != "" || instance.StatusCode != 103 {
		return core.ErrRecoveryRequired
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--", "/usr/bin/python3", "-I", "-c", hostOCILayoutVerify)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return fmt.Errorf("Host OCI layout differs from its managed source: %w", core.ErrRecoveryRequired)
	}
	return nil
}

// The optional maintained OCI integration supplies this callback; runtime tools
// remain optional and Core does not gain an OCI runtime dependency.
func (r *Runtime) ConfigureHostStorage(setup func(context.Context) error) {
	r.trustedHostStorage = setup
}
