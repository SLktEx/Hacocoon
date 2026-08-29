package incus

import (
	"context"
	"fmt"
	"strings"
)

const preparedImageAlias = "hacocoon-v0.1"
const imageBuilderName = "haco-image-builder-v0-1"
const nerdctlVersion = "2.3.5"
const nestedSmokeImage = "docker.io/library/alpine:3.22"

func (r *Runtime) ensureBaseImage(ctx context.Context, pool string) error {
	if _, err := r.runner.Run(ctx, "incus", "image", "show", preparedImageAlias, "--project", r.project); err == nil {
		return nil
	}

	// A fixed builder name makes interrupted init easy to recognize and clean up.
	// Ignore cleanup errors here: absence of the builder is the normal case.
	_, _ = r.runner.Run(ctx, "incus", "delete", imageBuilderName, "--project", r.project, "--force")
	defer func() {
		_, _ = r.runner.Run(context.WithoutCancel(ctx), "incus", "delete", imageBuilderName, "--project", r.project, "--force")
	}()

	args := []string{"launch", r.sourceImage, imageBuilderName, "--project", r.project, "-c", "security.nesting=true"}
	if pool != "" {
		args = append(args, "--storage", pool)
	}
	if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
		return fmt.Errorf("launch base-image builder: %w", err)
	}

	if _, err := r.runner.Run(ctx, "incus", "exec", imageBuilderName, "--project", r.project, "--", "sh", "-ceu", baseImageProvisionScript()); err != nil {
		return fmt.Errorf("provision base image: %w", err)
	}
	if _, err := r.runner.Run(ctx, "incus", "stop", imageBuilderName, "--project", r.project); err != nil {
		return fmt.Errorf("stop base-image builder: %w", err)
	}
	if _, err := r.runner.Run(ctx, "incus", "publish", imageBuilderName, "--project", r.project, "--alias", preparedImageAlias, "--reuse"); err != nil {
		return fmt.Errorf("publish base image: %w", err)
	}
	return nil
}

func baseImageProvisionScript() string {
	// nerdctl is pinned and checksum-verified. Ubuntu provides containerd and the
	// CNI plugins so systemd owns the daemon lifecycle instead of Hacocoon.
	return strings.TrimSpace(fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl tar containerd containernetworking-plugins
systemctl enable --now containerd

case "$(dpkg --print-architecture)" in
  amd64) nerdctl_arch=amd64 ;;
  arm64) nerdctl_arch=arm64 ;;
  *) echo "unsupported nerdctl architecture: $(dpkg --print-architecture)" >&2; exit 1 ;;
esac

asset="nerdctl-%[1]s-linux-${nerdctl_arch}.tar.gz"
release="https://github.com/containerd/nerdctl/releases/download/v%[1]s"
curl -fsSLo "/tmp/${asset}" "${release}/${asset}"
curl -fsSLo /tmp/nerdctl-SHA256SUMS "${release}/SHA256SUMS"
(
  cd /tmp
  grep -E "[[:space:]]${asset}$" nerdctl-SHA256SUMS | sha256sum -c -
)
tar -C /usr/local/bin -xzf "/tmp/${asset}" nerdctl
rm -f "/tmp/${asset}" /tmp/nerdctl-SHA256SUMS

mkdir -p /opt/cni
ln -sfn /usr/lib/cni /opt/cni/bin

systemctl is-active --quiet containerd
nerdctl --version
nerdctl info >/dev/null
nerdctl run --rm %[2]s true
`, nerdctlVersion, nestedSmokeImage))
}
