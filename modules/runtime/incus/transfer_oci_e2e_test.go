//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func transferOCIEnabled(t *testing.T) bool {
	t.Helper()
	if os.Getenv("HACO_E2E_TRANSFER_OCI") != "1" {
		t.Log("SKIP live containerd transfer: HACO_E2E_TRANSFER_OCI not enabled")
		return false
	}
	return true
}

func transferOCIGuest(t *testing.T, ctx context.Context, r *Runtime, ref, phase, script string) {
	t.Helper()
	result, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", script)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		t.Fatalf("containerd transfer fixture phase=%s exit=%d runner_failed=%t truncated=%t category=%s", phase, result.ExitCode, err != nil, result.StdoutTruncated || result.StderrTruncated, transferFailureCategory(result.Stderr))
	}
}

// Populate only the aggregate's newly owned, stopped source. The normal importer
// still has to configure its own nesting, network and Store attachment.
func prepareTransferOCI(t *testing.T, ctx context.Context, r *Runtime, ref, generation string) {
	t.Helper()
	if !transferOCIEnabled(t) {
		return
	}
	assets := os.Getenv("HACO_E2E_OCI_RUNTIME_ASSETS")
	if !filepath.IsAbs(assets) {
		t.Fatal("explicit absolute runtime assets required")
	}
	file, err := os.Open(filepath.Join(assets, "nerdctl-full.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	n, readErr := io.Copy(h, io.LimitReader(file, 512*1024*1024+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || n > 512*1024*1024 || fmt.Sprintf("%x", h.Sum(nil)) != "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa" {
		t.Fatal("invalid pinned runtime archive")
	}
	if err := r.VerifyEnvironmentIdentity(ctx, ref, generation); err != nil {
		t.Fatal(err)
	}
	run := func(phase string, args ...string) {
		t.Helper()
		result, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || result.ExitCode != 0 {
			t.Fatalf("prepare owned transfer runtime phase=%s exit=%d runner_failed=%t category=%s", phase, result.ExitCode, err != nil, transferFailureCategory(result.Stderr))
		}
	}
	run("enable-nesting", "config", "set", ref, "security.nesting", "true", "--project", r.project)
	run("start-source", "start", ref, "--project", r.project)
	transferOCIGuest(t, ctx, r, ref, "prepare-input-directory", "mkdir -m 700 /var/lib/haco-transfer-input")
	run("push-runtime", "file", "push", filepath.Join(assets, "nerdctl-full.tar.gz"), ref+"/var/lib/haco-transfer-input/runtime.tar.gz", "--project", r.project)
	run("push-probe", "file", "push", filepath.Join(assets, "oci-transfer-probe"), ref+"/var/lib/haco-transfer-input/probe", "--project", r.project)
	transferOCIGuest(t, ctx, r, ref, "configure-store", persistentOCIConfiguration)
	transferOCIGuest(t, ctx, r, ref, "install-runtime", transferContainerdSetup)
	transferOCIGuest(t, ctx, r, ref, "start-containerd", transferContainerdStart)
	transferOCIGuest(t, ctx, r, ref, "import-image", transferContainerdSeed)
	transferOCIGuest(t, ctx, r, ref, "inspect-image", transferContainerdImageIdentity)
	transferOCIGuest(t, ctx, r, ref, "run-container", transferContainerdRun)
	transferOCIGuest(t, ctx, r, ref, "quiesce-containerd", transferContainerdQuiesce)
	run("stop-source", "stop", ref, "--project", r.project, "--timeout", "30")
	t.Log("PASS source containerd image executed and stopped container writable data persisted before export")
}

func verifyTransferredOCI(t *testing.T, ctx context.Context, r *Runtime, ref string) {
	t.Helper()
	if !transferOCIEnabled(t) {
		return
	}
	transferOCIGuest(t, ctx, r, ref, "start-containerd", transferContainerdStart)
	transferOCIGuest(t, ctx, r, ref, "verify-container", transferContainerdVerify)
	t.Log("PASS imported containerd image identity and stopped container writable data; explicit start resumed saved work without task migration")
}

const transferContainerdSetup = `set -eu
# This offline fixture imports directly into native. The transfer service defaults
# to overlayfs; production's documented pull path instead defers unpack to run.
cat >> /etc/containerd/config.toml <<'UNPACK'
[[plugins."io.containerd.transfer.v1.local".unpack_config]]
  platform = "linux/amd64"
  snapshotter = "native"
  differ = "walking"
UNPACK
tar -xzf /var/lib/haco-transfer-input/runtime.tar.gz -C /usr/local bin/nerdctl bin/containerd bin/containerd-shim-runc-v2 bin/ctr bin/runc
rm /var/lib/haco-transfer-input/runtime.tar.gz
cat > /etc/systemd/system/containerd.service <<'UNIT'
[Unit]
Description=Owned transfer fixture containerd
[Service]
ExecStart=/usr/local/bin/containerd --config /etc/containerd/config.toml
Delegate=yes
KillMode=process
UNIT
systemctl daemon-reload
`
const transferContainerdStart = `set -eu
systemctl start containerd
attempt=0
until ctr version >/dev/null 2>&1; do
 attempt=$((attempt+1)); test "$attempt" -lt 60; sleep 0.5
done
`
const transferContainerdSeed = `set -eu
mkdir /var/lib/haco-transfer-input/root
cp /var/lib/haco-transfer-input/probe /var/lib/haco-transfer-input/root/probe
chmod 755 /var/lib/haco-transfer-input/root/probe
tar -cf /var/lib/haco-transfer-input/image.tar -C /var/lib/haco-transfer-input/root .
nerdctl --snapshotter native import --platform linux/amd64 /var/lib/haco-transfer-input/image.tar hacocoon-transfer:local
`
const transferContainerdImageIdentity = `set -eu
nerdctl --snapshotter native image inspect --format '{{.Id}}' hacocoon-transfer:local > /var/lib/haco-transfer-image-id
`
const transferContainerdRun = `set -eu
result=$(nerdctl --snapshotter native run --pull never --net none --name haco-transfer-persist hacocoon-transfer:local /probe) || { printf '%s\n' 'transfer container execution failed' >&2; exit 1; }
if [ "$result" != created ]; then printf '%s\n' 'transfer probe result mismatch' >&2; exit 1; fi
# nerdctl retains an exited task record for a named container. Absence of every
# task record is not the quiescence contract; the process must have exited.
test "$(nerdctl container inspect --format '{{.State.Status}}' haco-transfer-persist)" = exited
`
const transferContainerdQuiesce = `set -eu
rm /var/lib/haco-transfer-input/image.tar /var/lib/haco-transfer-input/probe /var/lib/haco-transfer-input/root/probe
rmdir /var/lib/haco-transfer-input/root /var/lib/haco-transfer-input
systemctl stop containerd
`
const transferContainerdVerify = `set -eu
test -z "$(ctr tasks list -q)"
test "$(nerdctl --snapshotter native image inspect --format '{{.Id}}' hacocoon-transfer:local)" = "$(cat /var/lib/haco-transfer-image-id)"
test "$(nerdctl --snapshotter native start --attach haco-transfer-persist)" = retained
test "$(nerdctl container inspect --format '{{.State.Status}}' haco-transfer-persist)" = exited
systemctl stop containerd
`

func TestTransferOCIShellSyntax(t *testing.T) {
	for name, script := range map[string]string{
		"setup": transferContainerdSetup, "start": transferContainerdStart,
		"seed": transferContainerdSeed, "identity": transferContainerdImageIdentity, "run": transferContainerdRun, "quiesce": transferContainerdQuiesce, "verify": transferContainerdVerify,
	} {
		t.Run(name, func(t *testing.T) {
			command := exec.Command("/bin/sh", "-n")
			command.Stdin = strings.NewReader(script)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("invalid owned fixture shell: %v %s", err, output)
			}
		})
	}
}

// Only fixed categories leave the fixture boundary; never print backend output.
func transferFailureCategory(stderr string) string {
	lower := strings.ToLower(stderr)
	for _, category := range []string{"permission denied", "no such file or directory", "not found", "does not exist", "not authorized", "connection refused", "no space left on device", "is not running", "read-only file system", "failed to create shim", "apparmor", "cgroup", "failed to extract layer", "no unpack platforms defined", "transfer container execution failed", "transfer probe result mismatch", "transfer exited task remains", "cni"} {
		if strings.Contains(lower, category) {
			return category
		}
	}
	return "unclassified"
}
func TestTransferFailureCategory(t *testing.T) {
	if transferFailureCategory("Error: open secret-path: no such file or directory") != "no such file or directory" {
		t.Fatal("missing fixed category")
	}
	if transferFailureCategory("credential=secret") != "unclassified" {
		t.Fatal("untrusted output exposed")
	}
}
