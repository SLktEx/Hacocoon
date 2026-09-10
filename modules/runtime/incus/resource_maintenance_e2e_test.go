package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// This native primitive test does not claim public detached-image acceptance.
func TestRealIncusResourceMaintenancePreparationE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_RESOURCE_MAINTENANCE") != "1" {
		t.Skip("opt in on a dedicated root Incus/Btrfs host with cached image/pool")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("explicit root pool and full cached image required")
	}
	duration := 5 * time.Minute
	if os.Getenv("HACO_E2E_MAINTENANCE_CONTROLLER") != "" {
		duration = 20 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	r := New(host.ExecRunner{})
	p, err := NewSandboxProvider(r)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(nonce[:])
	ref := "haco-maintenance-" + owner[:16]
	instance, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	resource := core.PersistentResource{ID: "oci:maintenance-" + owner[:16], Owner: owner, Kind: OCIStoreKind, NativeRef: pool + "/haco-persistent-" + owner, State: "ready", CreatedAt: time.Now().UTC()}
	dir, err := os.MkdirTemp("/var/lib", "haco-maintenance-")
	if err != nil {
		t.Fatal(err)
	}
	receipt := struct {
		Project, Instance, Generation string
		Resource                      core.PersistentResource
	}{r.project, ref, instance, resource}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "ownership.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated instance %s and Store %s; receipt %s; failure retains owned resources", ref, resource.NativeRef, dir)
	command := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
			phase := "native-call"
			for _, line := range strings.Split(out.Stdout, "\n") {
				switch line {
				case "HACO_MAINTENANCE_PHASE=install", "HACO_MAINTENANCE_PHASE=daemon", "HACO_MAINTENANCE_PHASE=import", "HACO_MAINTENANCE_PHASE=container":
					phase = strings.TrimPrefix(line, "HACO_MAINTENANCE_PHASE=")
				}
			}
			t.Fatalf("Incus fixture command failed at %s (exit %d): %v", phase, out.ExitCode, err)
		}
		return out.Stdout
	}
	command("init", "local:"+image, ref, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+instance, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue)
	command("start", ref, "--project", r.project)
	command("exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", `n=0; until systemctl show --property=Version --value >/dev/null 2>&1; do n=$((n+1)); test "$n" -lt 60; sleep 0.5; done
mkdir -p /var/lib/hacocoon-oci
cat > /etc/systemd/system/docker.service <<'UNIT'
[Unit]
Description=Isolated maintenance guard fixture
DefaultDependencies=no
[Service]
Type=oneshot
ExecStart=/usr/bin/touch /var/lib/hacocoon-oci/unwanted-start
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
timeout 30 systemctl enable --now docker.service
test -f /var/lib/hacocoon-oci/unwanted-start`)
	if err := p.prepareResourceMaintenance(ctx, ref); err != nil {
		t.Fatal(err)
	}
	installNativeMaintenanceTooling(t, ctx, p, ref, instance, dir)
	backend := &PersistentResourceBackend{Runtime: r}
	if err := backend.Create(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if err := backend.Verify(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if err := p.attachPersistentResource(ctx, ref, resource); err != nil {
		t.Fatal(err)
	}
	command("exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", "printf retained > /var/lib/hacocoon-oci/sentinel; test ! -e /var/lib/hacocoon-oci/unwanted-start")
	verifyContainerdMaintenanceRuntime(t, ctx, p, ref, instance, resource, command)
	if err := p.prepareResourceMaintenance(ctx, ref); err == nil {
		t.Fatal("preparation accepted after Store attachment")
	}
	command("restart", ref, "--project", r.project)
	command("exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", `n=0; until systemctl show --property=Version --value >/dev/null 2>&1; do n=$((n+1)); test "$n" -lt 60; sleep 0.5; done
test "$(cat /var/lib/hacocoon-oci/sentinel)" = retained
test ! -e /var/lib/hacocoon-oci/unwanted-start
test "$(systemctl show --property=LoadState --value docker.service)" = masked`)
	if err := r.VerifyEnvironmentIdentity(ctx, ref, instance); err != nil {
		t.Fatal(err)
	}
	command("delete", ref, "--project", r.project, "--force")
	if exists, err := r.environmentExists(ctx, ref); err != nil || exists {
		t.Fatal("runtime absence unproven", err)
	}
	if err := backend.Verify(ctx, resource); err != nil {
		t.Fatal("Store lost with runtime", err)
	}
	if verifyMaintenanceControllerCLI(t, ctx, r, resource, dir) {
		return
	}
	if err := backend.Delete(ctx, resource); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS preparation before attach, late-preparation refusal, masked restart, retained Store after runtime deletion and exact owned cleanup; shared image/pool and receipt retained")
}
