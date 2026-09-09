package incus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

// No Base or cached image participates: the source is an empty stopped container.
// This tests data transport and config separation, not a bootable public import.
func TestRealIncusRootfsTransferE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_ROOTFS_TRANSFER") != "1" {
		t.Skip("requires explicit dedicated root Incus/Btrfs acceptance")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	run := func(args ...string) string {
		t.Helper()
		out, err := runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("Incus fixture %v failed: %v", args, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	var n [16]byte
	if _, err := rand.Read(n[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(n[:])
	pool := "haco-transfer-" + owner[:16]
	project := pool
	source, target := "source", "destination"
	alias := "rootfs-" + owner
	dir, err := os.MkdirTemp("/var/lib", "haco-rootfs-transfer-")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.OpenFile(filepath.Join(dir, "plan.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(receipt).Encode(map[string]string{"pool": pool, "project": project, "source": source, "target": target, "alias": alias, "owner": owner}); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("exact owned fixture and retained archive", dir)
	const key = "user.hacocoon.transfer-test"
	run("storage", "create", pool, "btrfs", "size=1GiB", key+"="+owner)
	run("project", "create", project, "-c", "features.images=true", "-c", key+"="+owner)
	run("init", "--empty", source, "--project", project, "--no-profiles", "--storage", pool, "--config", key+"="+owner, "--config", "boot.autostart=false", "--config", "environment.OLD_TOKEN=synthetic-fixture-only", "--config", environmentInstanceKey+"="+owner)
	marker := filepath.Join(dir, "marker")
	if err := os.WriteFile(marker, []byte("explicit rootfs bytes without a Base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("file", "push", marker, source+"/root/retained", "--create-dirs", "--project", project)
	run("publish", source, "--alias", alias, "--compression=none", key+"="+owner, "--project", project)
	var image struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal([]byte(run("query", "/1.0/images/aliases/"+alias+"?project="+project)), &image); err != nil || !baseFingerprintPattern.MatchString(image.Target) {
		t.Fatal("invalid publication identity", err)
	}
	// Native image properties and the random alias durably identify publication.
	imageOwner := func() {
		t.Helper()
		var info struct {
			Properties map[string]string `json:"properties"`
		}
		if err := json.Unmarshal([]byte(run("query", "/1.0/images/"+image.Target+"?project="+project)), &info); err != nil || info.Properties[key] != owner {
			t.Fatal("foreign image")
		}
	}
	imageOwner()
	run("image", "export", image.Target, filepath.Join(dir, "rootfs"), "--project", project)
	entries, err := filepath.Glob(filepath.Join(dir, "rootfs*"))
	if err != nil || len(entries) != 1 {
		t.Fatal("expected one unified rootfs image archive", err)
	}
	archive := entries[0]
	bytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(bytes)
	if run("config", "get", source, key, "--project", project) != owner {
		t.Fatal("foreign source")
	}
	run("delete", source, "--project", project)
	imageOwner()
	run("image", "delete", image.Target, "--project", project)
	var images []any
	if err := json.Unmarshal([]byte(run("image", "list", "--format=json", "--project", project)), &images); err != nil || len(images) != 0 {
		t.Fatal("source image not positively absent", err)
	}
	run("image", "import", archive, "--alias", alias, "--project", project)
	var imported struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal([]byte(run("query", "/1.0/images/aliases/"+alias+"?project="+project)), &imported); err != nil || imported.Target != image.Target {
		t.Fatal("archive fingerprint changed", err)
	}
	newID, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	run("init", "local:"+image.Target, target, "--project", project, "--no-profiles", "--storage", pool, "--config", key+"="+owner, "--config", environmentInstanceKey+"="+newID, "--config", "boot.autostart=false", "--config", "security.privileged=false", "--config", "security.nesting=false")
	var observed snapshotInstanceObservation
	if err := json.Unmarshal([]byte(run("query", "/1.0/instances/"+target+"?project="+project)), &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Status != "Stopped" && observed.Status != "STOPPED" {
		t.Fatal("import unexpectedly running")
	}
	if len(observed.Profiles) != 0 || len(observed.ExpandedDevices) != 1 || observed.ExpandedConfig["environment.OLD_TOKEN"] != "" || observed.Config[environmentInstanceKey] != newID {
		t.Fatal("source configuration/identity inherited")
	}
	output := filepath.Join(dir, "readback")
	run("file", "pull", target+"/root/retained", output, "--project", project)
	restored, err := os.ReadFile(output)
	if err != nil || string(restored) != "explicit rootfs bytes without a Base\n" {
		t.Fatal("rootfs bytes lost", err)
	}
	// Drop the imported transport image while the newly created rootfs remains.
	imageOwner()
	run("image", "delete", image.Target, "--project", project)
	if run("config", "get", target, key, "--project", project) != owner {
		t.Fatal("foreign destination")
	}
	run("delete", target, "--project", project)
	if run("project", "get", project, key) != owner {
		t.Fatal("foreign project")
	}
	run("project", "delete", project)
	if run("storage", "get", pool, key) != owner {
		t.Fatal("foreign pool")
	}
	var volumes []any
	if err := json.Unmarshal([]byte(run("storage", "volume", "list", pool, "--all-projects", "--format=json")), &volumes); err != nil || len(volumes) != 0 {
		t.Fatal("pool not empty", err)
	}
	run("storage", "delete", pool)
	after, err := os.ReadFile(archive)
	if err != nil || sha256.Sum256(after) != before {
		t.Fatal("archive changed", err)
	}
	t.Logf("rootfs image round trip passed; source instance/image removed before import; archive %s sha256 %x; no Base, cache, startup, Workspace/OCI or public importer used", archive, before)
}
