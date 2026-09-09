package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Optional real runtime extension of the owned-area fixture. The Host builds and
// uses the images before copying; no image export/import participates in copying.
func prepareHostRuntimeCopy(t *testing.T, ctx context.Context, runtime *Runtime, source core.PersistentResource, command func(...string) string) func(core.PersistentResource) {
	t.Helper()
	assets := os.Getenv("HACO_E2E_OCI_RUNTIME_ASSETS")
	if assets == "" {
		return nil
	}
	if !filepath.IsAbs(assets) {
		t.Fatal("runtime assets must use an absolute directory")
	}
	for name, expected := range map[string]string{
		"nerdctl-full.tar.gz": "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa",
		"docker.tgz":          "ea90cfd12e1eeb12aa1c971741adb8bd4ed88e2a574eaac13f5029a1dbc6300d",
	} {
		file, err := os.Open(filepath.Join(assets, name))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(file, 512*1024*1024+1))
		file.Close()
		if err != nil || n > 512*1024*1024 || fmt.Sprintf("%x", h.Sum(nil)) != expected {
			t.Fatal("invalid runtime asset", name)
		}
	}
	guest := func(name, script string) string {
		t.Helper()
		return command("exec", name, "--project", runtime.project, "--", "/bin/sh", "-ec", script)
	}
	install := func(name string) {
		for _, file := range []string{"nerdctl-full.tar.gz", "docker.tgz", "oci-probe"} {
			command("file", "push", filepath.Join(assets, file), name+"/tmp/"+file, "--project", runtime.project)
		}
		guest(name, runtimeFixtureInstall)
	}
	imageID := func(name, tool string) string {
		id := strings.TrimSpace(guest(name, tool+` image inspect --format '{{.Id}}' hacocoon-area:local`))
		if !strings.HasPrefix(id, "sha256:") || len(id) != 71 {
			t.Fatal("image identity malformed")
		}
		return id
	}
	runImage := func(name, tool string) {
		if strings.TrimSpace(guest(name, tool+" run --rm --pull never --network none hacocoon-area:local")) != "hacocoon-area-image-ok" {
			t.Fatal("local image did not execute")
		}
	}
	install(trustedHostName)
	guest(trustedHostName, `mkdir /tmp/area-context
install -m 755 /tmp/oci-probe /tmp/area-context/probe
printf 'FROM scratch\nCOPY probe /probe\nENTRYPOINT ["/probe"]\n' > /tmp/area-context/Dockerfile
DOCKER_BUILDKIT=0 /opt/docker/docker build --network none -t hacocoon-area:local /tmp/area-context
nerdctl --snapshotter native build --network none -t hacocoon-area:local /tmp/area-context`)
	tools := []string{"/opt/docker/docker", "nerdctl --snapshotter native"}
	ids := map[string]string{}
	for _, tool := range tools {
		ids[tool] = imageID(trustedHostName, tool)
		runImage(trustedHostName, tool)
	}
	t.Log("PASS Host Docker and nerdctl locally built images execute before area copy")
	guest(trustedHostName, `printf '\nLABEL hacocoon.fixture=host-image-delete\n' >> /tmp/area-context/Dockerfile
DOCKER_BUILDKIT=0 /opt/docker/docker build --network none -t hacocoon-host-delete:Dev /tmp/area-context
nerdctl --snapshotter native build --network none -t hacocoon-host-delete:Dev /tmp/area-context`)
	for _, tool := range tools {
		verifyHostSourceImages(t, ctx, runtime, source, tool, guest)
		if imageID(trustedHostName, tool) != ids[tool] {
			t.Fatal("Host source deletion changed the retained image")
		}
		runImage(trustedHostName, tool)
	}

	return func(target core.PersistentResource) {
		name := "haco-area-runtime-copy"
		command("launch", defaultImage, name, "--project", runtime.project, "--storage", strings.Split(target.NativeRef, "/")[0], "--no-profiles", "--config", "user.hacocoon.kind=runtime-copy-fixture")
		provider, err := NewSandboxProvider(runtime)
		if err != nil {
			t.Fatal(err)
		}
		if err := provider.attachPersistentResource(ctx, name, target); err != nil {
			t.Fatal(err)
		}
		guest(name, persistentOCIConfiguration)
		install(name)
		for _, tool := range tools {
			if imageID(name, tool) != ids[tool] {
				t.Fatal("copied image identity changed")
			}
			runImage(name, tool)
			verifyManagedRuntimeImages(t, ctx, runtime, name, target, tool, guest)
			if imageID(trustedHostName, tool) != ids[tool] {
				t.Fatal("copy deletion changed Host image")
			}
			runImage(trustedHostName, tool)
		}
		t.Log("PASS offline Docker/nerdctl area recovery, image identity and independent image deletion")
		// The fixture was freshly created in this random owned project. Verify its
		// marker and exact volume before deleting, then leave catalog cleanup to caller.
		if strings.TrimSpace(command("config", "get", name, "user.hacocoon.kind", "--project", runtime.project)) != "runtime-copy-fixture" {
			t.Fatal("fixture ownership changed")
		}
		pool, volume, err := persistentVolume(target)
		if err != nil {
			t.Fatal(err)
		}
		for key, expected := range map[string]string{"pool": pool, "source": volume, "path": OCIStorePath} {
			if strings.TrimSpace(command("config", "device", "get", name, "persistent-resource", key, "--project", runtime.project)) != expected {
				t.Fatal("fixture attachment changed")
			}
		}
		command("delete", name, "--force", "--project", runtime.project)
	}
}

const runtimeFixtureInstall = `set -eu
mkdir -p /opt/docker /usr/local/bin /etc/systemd/system
# Extract only fixed required runtime files from digest-verified distributions.
tar -xzf /tmp/nerdctl-full.tar.gz -C /usr/local bin/nerdctl bin/containerd bin/containerd-shim-runc-v2 bin/ctr bin/runc bin/buildkitd bin/buildctl
tar -xzf /tmp/docker.tgz -C /opt/docker --strip-components=1
ln -s /opt/docker/docker /usr/local/bin/docker
cat > /etc/systemd/system/containerd.service <<'UNIT'
[Unit]
Description=Owned test fixture containerd
[Service]
ExecStart=/usr/local/bin/containerd
Delegate=yes
KillMode=process
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/area-docker.service <<'UNIT'
[Unit]
Description=Owned offline test fixture Docker
[Service]
Environment=PATH=/opt/docker:/usr/local/bin:/usr/bin:/bin
ExecStart=/opt/docker/dockerd --iptables=false --ip6tables=false --bridge=none --storage-driver=vfs
Delegate=yes
KillMode=process
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl start containerd buildkit area-docker
attempt=0
until /opt/docker/docker info >/dev/null 2>&1 && nerdctl info >/dev/null 2>&1 && buildctl debug workers >/dev/null 2>&1; do
 attempt=$((attempt+1)); test "$attempt" -lt 60; sleep 1
done
/opt/docker/docker --version
nerdctl --version
containerd --version
buildkitd --version
`
