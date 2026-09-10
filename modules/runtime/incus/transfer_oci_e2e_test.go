//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func transferOCIGuest(t *testing.T, ctx context.Context, r *Runtime, ref, script string) {
	t.Helper()
	result, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", script)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		t.Fatal("containerd transfer fixture command failed", err)
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
	run := func(args ...string) {
		t.Helper()
		result, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || result.ExitCode != 0 {
			t.Fatal("prepare owned transfer runtime", err)
		}
	}
	run("config", "set", ref, "security.nesting", "true", "--project", r.project)
	run("start", ref, "--project", r.project)
	run("file", "push", filepath.Join(assets, "nerdctl-full.tar.gz"), ref+"/tmp/haco-transfer-runtime.tar.gz", "--project", r.project)
	run("file", "push", filepath.Join(assets, "oci-transfer-probe"), ref+"/tmp/haco-transfer-probe", "--project", r.project)
	transferOCIGuest(t, ctx, r, ref, persistentOCIConfiguration)
	transferOCIGuest(t, ctx, r, ref, transferContainerdSetup)
	transferOCIGuest(t, ctx, r, ref, transferContainerdStart)
	transferOCIGuest(t, ctx, r, ref, transferContainerdSeed)
	run("stop", ref, "--project", r.project, "--timeout", "30")
	t.Log("PASS source containerd image executed and stopped container writable data persisted before export")
}

func verifyTransferredOCI(t *testing.T, ctx context.Context, r *Runtime, ref string) {
	t.Helper()
	if !transferOCIEnabled(t) {
		return
	}
	transferOCIGuest(t, ctx, r, ref, transferContainerdStart)
	transferOCIGuest(t, ctx, r, ref, transferContainerdVerify)
	t.Log("PASS imported containerd image identity and stopped container writable data; explicit start resumed saved work without task migration")
}

const transferContainerdSetup = "set -eu\ntar -xzf /tmp/haco-transfer-runtime.tar.gz -C /usr/local bin/nerdctl bin/containerd bin/containerd-shim-runc-v2 bin/ctr bin/runc\nrm /tmp/haco-transfer-runtime.tar.gz\ncat > /etc/systemd/system/containerd.service <<'UNIT'\n[Unit]\nDescription=Owned transfer fixture containerd\n[Service]\nExecStart=/usr/local/bin/containerd --config /etc/containerd/config.toml\nDelegate=yes\nKillMode=process\nUNIT\nsystemctl daemon-reload\n"
const transferContainerdStart = "set -eu\nsystemctl start containerd\nattempt=0\nuntil ctr version >/dev/null 2>&1; do\n attempt=$((attempt+1)); test \"$attempt\" -lt 60; sleep 0.5\ndone\n"
const transferContainerdSeed = "set -eu\nmkdir /tmp/haco-transfer-root\ncp /tmp/haco-transfer-probe /tmp/haco-transfer-root/probe\nchmod 755 /tmp/haco-transfer-root/probe\ntar -cf /tmp/haco-transfer-image.tar -C /tmp/haco-transfer-root .\nnerdctl --snapshotter native import /tmp/haco-transfer-image.tar hacocoon-transfer:local\nnerdctl --snapshotter native image inspect --format '{{.Id}}' hacocoon-transfer:local > /var/lib/haco-transfer-image-id\ntest \"$(nerdctl --snapshotter native run --pull never --net none --name haco-transfer-persist hacocoon-transfer:local /probe)\" = created\ntest -z \"$(ctr tasks list -q)\"\nrm /tmp/haco-transfer-image.tar /tmp/haco-transfer-probe /tmp/haco-transfer-root/probe\nrmdir /tmp/haco-transfer-root\nsystemctl stop containerd\n"
const transferContainerdVerify = "set -eu\ntest -z \"$(ctr tasks list -q)\"\ntest \"$(nerdctl --snapshotter native image inspect --format '{{.Id}}' hacocoon-transfer:local)\" = \"$(cat /var/lib/haco-transfer-image-id)\"\ntest \"$(nerdctl --snapshotter native start --attach haco-transfer-persist)\" = retained\ntest -z \"$(ctr tasks list -q)\"\nsystemctl stop containerd\n"
