package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func verifyContainerdMaintenanceRuntime(t *testing.T, ctx context.Context, p *SandboxProvider, ref, generation string, resource core.PersistentResource, command func(...string) string) {
	t.Helper()
	assets := os.Getenv("HACO_E2E_OCI_RUNTIME_ASSETS")
	if assets == "" {
		t.Log("SKIP actual containerd maintenance: HACO_E2E_OCI_RUNTIME_ASSETS absent")
		return
	}
	if !filepath.IsAbs(assets) {
		t.Fatal("absolute fixture assets required")
	}
	file, err := os.Open(filepath.Join(assets, "nerdctl-full.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(file, 512<<20+1))
	file.Close()
	if err != nil || n > 512<<20 || fmt.Sprintf("%x", h.Sum(nil)) != "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa" {
		t.Fatal("invalid native fixture archive")
	}
	for _, name := range []string{"nerdctl-full.tar.gz", "oci-probe"} {
		command("file", "push", filepath.Join(assets, name), ref+"/tmp/"+name, "--project", p.project)
	}
	guest := func(script string) string {
		t.Helper()
		return command("exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", `exec /bin/sh -ec "$1" 2>/var/lib/haco-maintenance-fixture.stderr`, "fixture", script)
	}
	guest(containerdMaintenanceFixture)
	if err := p.startContainerdMaintenance(ctx, ref, generation, resource); err != nil {
		t.Fatal(err)
	}
	guest(`set -eu
ctr=/usr/local/bin/ctr
sock=/run/hacocoon-maintenance/containerd.sock
$ctr --address "$sock" plugins list > /var/lib/haco-maintenance-plugins
if awk '$NF == "ok"' /var/lib/haco-maintenance-plugins | grep -E 'restart|tasks-service|[[:space:]]tasks[[:space:]]|[[:space:]]task[[:space:]]|cri|nri|podsandbox'; then exit 50; fi
if $ctr --address "$sock" --namespace default tasks list >/dev/null 2>&1; then exit 52; fi
$ctr --address "$sock" --namespace default containers info retained-container > /tmp/container-after.json
cmp /tmp/container-before.json /tmp/container-after.json
$ctr --address "$sock" --namespace default images list --quiet | grep -Fx docker.io/library/maintenance-used:local
$ctr --address "$sock" --namespace default images remove docker.io/library/maintenance-unused:local
$ctr --address "$sock" --namespace default images list --quiet > /tmp/images-after
if grep -Fx docker.io/library/maintenance-unused:local /tmp/images-after; then exit 51; fi
grep -Fx docker.io/library/maintenance-used:local /tmp/images-after
sleep 12
$ctr --address "$sock" --namespace default containers info retained-container > /tmp/container-later.json
cmp /tmp/container-before.json /tmp/container-later.json
systemctl stop hacocoon-maintenance-containerd
`)
	t.Log("PASS real containerd 2.3.3 maintenance: task/restart/CRI disabled, restart-marked container metadata unchanged, used image retained and unused image removed")
}

const containerdMaintenanceFixture = `set -eu
printf "HACO_MAINTENANCE_PHASE=install\n"
tar -xzf /tmp/nerdctl-full.tar.gz -C /usr/local bin/containerd bin/ctr bin/nerdctl
mkdir /var/lib/hacocoon-oci/containerd
mkdir /tmp/maintenance-root
cp /tmp/oci-probe /tmp/maintenance-root/oci-probe
tar -cf /tmp/maintenance-root.tar -C /tmp/maintenance-root .
cat > /run/haco-fixture-containerd.toml <<'CONFIG'
version = 3
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/haco-fixture-containerd"
disabled_plugins = ["io.containerd.monitor.container.v1.restart", "io.containerd.cri.v1.images", "io.containerd.cri.v1.runtime", "io.containerd.nri.v1.nri"]
[grpc]
 address = "/run/haco-fixture-containerd.sock"
[[plugins."io.containerd.transfer.v1.local".unpack_config]]
 platform = "linux/amd64"
 snapshotter = "native"
CONFIG
printf "HACO_MAINTENANCE_PHASE=daemon\n"
systemd-run --unit=haco-fixture-containerd --property=Restart=no /usr/local/bin/containerd --config /run/haco-fixture-containerd.toml
n=0
until ctr --address /run/haco-fixture-containerd.sock version >/dev/null 2>&1; do n=$((n+1)); test "$n" -lt 60; sleep 0.5; done
printf "HACO_MAINTENANCE_PHASE=import\n"
nerdctl --address /run/haco-fixture-containerd.sock --snapshotter native import /tmp/maintenance-root.tar maintenance-used:local
ctr --address /run/haco-fixture-containerd.sock --namespace default images tag docker.io/library/maintenance-used:local docker.io/library/maintenance-unused:local
printf "HACO_MAINTENANCE_PHASE=container\n"
ctr --address /run/haco-fixture-containerd.sock --namespace default containers create --label containerd.io/restart.policy=always --label containerd.io/restart.status=running docker.io/library/maintenance-used:local retained-container /oci-probe
ctr --address /run/haco-fixture-containerd.sock --namespace default containers info retained-container > /tmp/container-before.json
systemctl stop haco-fixture-containerd
`
