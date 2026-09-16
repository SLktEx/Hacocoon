package incus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Extends the canonical Host-area copy fixture with the shipped provisioner.
// The receiver has no NIC: image use must succeed without registry access.
func prepareStandardHostToolingCopy(t *testing.T, ctx context.Context, runtime *Runtime, source core.PersistentResource, command func(...string) string) func(core.PersistentResource) {
	t.Helper()
	backend := &PersistentResourceBackend{Runtime: runtime}
	if err := backend.ProvisionHostTools(ctx, source); err != nil {
		// Report only fixed probe exit codes, never guest/package output.
		for _, probe := range []struct{ name, script string }{
			{"busctl_present", "test -x /usr/bin/busctl"},
			{"system_bus_socket", "test -S /run/dbus/system_bus_socket"},
			{"system_manager", "/usr/bin/busctl --system --timeout=2s get-property org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager Version"},
			{"private_manager", "/usr/bin/systemctl show --property=Version"},
			{"guest_ipv4", "ip -4 -o address show dev eth0 | grep -q 'inet '"},
			{"archive_dns", "timeout 5 getent ahostsv4 archive.ubuntu.com"},
		} {
			result, runErr := runtime.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", runtime.project, "--", "/bin/sh", "-ec", probe.script)
			t.Logf("Host tooling probe %s: exit=%d runner_error=%t", probe.name, result.ExitCode, runErr != nil)
		}
		t.Fatal(err)
	}
	guest := func(name, script string) string {
		t.Helper()
		return command("exec", name, "--project", runtime.project, "--", "/bin/sh", "-ec", script)
	}
	guest(trustedHostName, `git --version
gh --version
nerdctl --version
buildctl --version
test ! -e /usr/local/bin/docker
test ! -e /usr/bin/docker
systemctl restart containerd buildkit`)
	guest(trustedHostName, `nerdctl pull docker.io/library/busybox:latest`)
	guest(trustedHostName, `nerdctl run --rm docker.io/library/busybox:latest echo public-pull-ok`)
	guest(trustedHostName, `mkdir -p /root/host-tooling-context
printf 'FROM busybox:latest\nRUN echo built > /built\nCMD ["cat", "/built"]\n' > /root/host-tooling-context/Dockerfile`)
	guest(trustedHostName, `nerdctl build -t hacocoon-standard:local /root/host-tooling-context`)
	guest(trustedHostName, `test "$(nerdctl run --rm --pull never hacocoon-standard:local)" = built
test -s /var/lib/hacocoon-oci/buildkit/cache.db
buildctl du -v | grep '^ID:' | sort > /root/build-cache-before
test -s /root/build-cache-before`)
	id := strings.TrimSpace(guest(trustedHostName, `nerdctl image inspect --format '{{.Id}}' hacocoon-standard:local`))
	if !strings.HasPrefix(id, "sha256:") {
		t.Fatal("missing built image identity")
	}
	command("stop", trustedHostName, "--project", runtime.project, "--timeout", "60")
	command("start", trustedHostName, "--project", runtime.project)
	if err := backend.ProvisionHostTools(ctx, source); err != nil {
		t.Fatal("repeat setup", err)
	}
	if after := strings.TrimSpace(guest(trustedHostName, `nerdctl image inspect --format '{{.Id}}' hacocoon-standard:local`)); after != id {
		t.Fatal("repeat setup lost image")
	}
	guest(trustedHostName, `test "$(nerdctl run --rm --pull never hacocoon-standard:local)" = built
buildctl du -v | grep '^ID:' | sort > /root/build-cache-after
cmp /root/build-cache-before /root/build-cache-after
nerdctl build -t hacocoon-standard:repeat /root/host-tooling-context`)
	t.Log("PASS standard Git/gh, public pull/run, BuildKit build/run, restart and repeat setup retaining image/cache")

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
		// Transfer only the digest-verified public binary archive into this offline
		// fixture, never a Host configuration directory, socket or credential.
		archive := "/var/cache/hacocoon/host-tooling/nerdctl-full-2.3.5-linux-amd64.tar.gz"
		local := filepath.Join(t.TempDir(), "nerdctl-full.tar.gz")
		command("file", "pull", trustedHostName+archive, local, "--project", runtime.project)
		guest(name, "mkdir -p /var/cache/hacocoon/host-tooling")
		command("file", "push", local, name+archive, "--project", runtime.project, "--mode", "0644")
		for _, stage := range []string{"host_tooling", "host_services"} {
			command("exec", name, "--project", runtime.project, "--", "/usr/bin/python3", "-I", "-c", hostToolingScript, stage)
		}
		if copied := strings.TrimSpace(guest(name, `nerdctl image inspect --format '{{.Id}}' hacocoon-standard:local`)); copied != id {
			t.Fatal("copy changed image identity")
		}
		guest(name, `test "$(nerdctl run --rm --network none --pull never hacocoon-standard:local)" = built
nerdctl image rm hacocoon-standard:local
test ! -S /run/hacocoon/control.sock
test ! -S /var/lib/incus/unix.socket`)
		guest(trustedHostName, `test "$(nerdctl run --rm --pull never hacocoon-standard:local)" = built`)
		t.Log("PASS managed Host area copied to independent offline Store; image reused without pull and deletion isolated")
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

func TestRealIncusHostToolingE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_HOST_TOOLING") != "1" {
		t.Skip("set HACO_E2E_HOST_TOOLING=1 on a dedicated root Incus/Btrfs host")
	}
	t.Setenv("HACO_E2E_INCUS_HOST_AREA_COPY", "1")
	testRealIncusHostAreaCopy(t, false)
}
