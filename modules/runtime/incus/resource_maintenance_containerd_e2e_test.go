package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
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
	verifyMetadataImageOperations(t, ctx, p.Runtime, ref, generation, resource)
	guest(`set -eu
ctr=/usr/local/bin/ctr
sock=/run/hacocoon-maintenance/containerd.sock
$ctr --address "$sock" plugins list > /var/lib/haco-maintenance-plugins
if awk '$NF == "ok"' /var/lib/haco-maintenance-plugins | grep -E 'restart|tasks-service|[[:space:]]tasks[[:space:]]|[[:space:]]task[[:space:]]|cri|nri|podsandbox'; then exit 50; fi
if $ctr --address "$sock" --namespace default tasks list >/dev/null 2>&1; then exit 52; fi
$ctr --address "$sock" --namespace default containers info aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa > /tmp/container-after.json
cmp /tmp/container-before.json /tmp/container-after.json
$ctr --address "$sock" --namespace default images list --quiet | grep -Fx docker.io/library/maintenance-used:local
$ctr --address "$sock" --namespace default images list --quiet > /tmp/images-after
grep -Fx docker.io/library/maintenance-used:local /tmp/images-after
sleep 12
$ctr --address "$sock" --namespace default containers info aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa > /tmp/container-later.json
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
printf 'independent unused image\n' > /tmp/maintenance-root/unused
 tar -cf /tmp/maintenance-unused.tar -C /tmp/maintenance-root .
 nerdctl --address /run/haco-fixture-containerd.sock --snapshotter native import /tmp/maintenance-unused.tar maintenance-unused:local
printf "HACO_MAINTENANCE_PHASE=container\n"
ctr --address /run/haco-fixture-containerd.sock --namespace default containers create --label containerd.io/restart.policy=always --label containerd.io/restart.status=running docker.io/library/maintenance-used:local aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa /oci-probe
ctr --address /run/haco-fixture-containerd.sock --namespace default containers info aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa > /tmp/container-before.json
systemctl stop haco-fixture-containerd
`

// This fixture supplies catalog/lifecycle identities; execution and OCI formats
// are native. It does not assert installed-controller creation or provisioning.
func verifyMetadataImageOperations(t *testing.T, ctx context.Context, runtime *Runtime, name, generation string, resource core.PersistentResource) {
	t.Helper()
	f := &runtimeImageFixture{runtime: runtime, name: name, resource: resource, instance: generation}
	service := &oci.ManagedImages{Catalog: f, Environments: f}
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	service.Maintain = func(ctx context.Context, ref core.PersistentResourceRef, action func(context.Context, core.Environment) error) error {
		if ref != resource.Ref() {
			return core.ErrCapabilityStale
		}
		return action(ctx, core.Environment{Name: name, Workspace: work, PersistentResource: ref})
	}
	all, err := service.List(ctx, resource.ID, "nerdctl")
	if err != nil {
		t.Fatal("metadata product inventory", err)
	}
	if !all.Target.Detached || len(all.Images) < 2 {
		t.Fatal("invalid metadata inventory", len(all.Images))
	}
	used, referenced, unused := map[string]bool{}, map[string]bool{}, ""
	for _, img := range all.Images {
		classified := false
		for _, tag := range img.Tags {
			if strings.HasSuffix(tag, "maintenance-used:local") {
				used[img.ID] = true
				if len(img.Containers) > 0 {
					referenced[img.ID] = true
				}
				classified = true
			}
			if strings.HasSuffix(tag, "maintenance-unused:local") {
				if unused == "" {
					unused = img.ID
				}
				classified = true
			}
		}
		if !classified {
			t.Fatal("unexpected fixture image", img.ID)
		}
	}
	if len(used) == 0 || len(referenced) == 0 || unused == "" || used[unused] {
		t.Fatal("independent fixture digests required")
	}
	for id := range referenced {
		if err := service.Delete(ctx, all.Target, id); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal("retained container image not protected", err)
		}
	}
	if err := service.Delete(ctx, all.Target, unused); err != nil {
		t.Fatal("metadata product deletion", err)
	}
	after, err := service.List(ctx, resource.ID, "nerdctl")
	if err != nil {
		t.Fatal("metadata image absence unproven", err)
	}
	for _, img := range after.Images {
		if img.ID == unused {
			t.Fatal("selected digest still present")
		}
		if used[img.ID] {
			if referenced[img.ID] && len(img.Containers) != 1 {
				t.Fatal("container reference lost")
			}
			delete(used, img.ID)
		}
	}
	if len(used) != 0 {
		t.Fatal("used image digest lost")
	}
	t.Log("PASS product image operations on native metadata socket: inventory, referenced-image refusal, independent unused digest deletion and confirmed absence")
}
