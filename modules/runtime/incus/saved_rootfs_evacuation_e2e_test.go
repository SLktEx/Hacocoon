//go:build linux

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

// This evacuates only a freshly owned, stopped, saved rootfs. It is not an
// arbitrary archive importer or a complete installation backup.
func TestRealIncusSavedRootfsEvacuationE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SAVED_ROOTFS_EVACUATION") != "1" {
		t.Skip("requires dedicated root Incus/Btrfs acceptance")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	run := func(command string, args ...string) string {
		t.Helper()
		out, err := runner.Run(ctx, command, args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("fixture %s %v failed (exit %d): %v", command, args, out.ExitCode, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(random[:])
	pool, project := "haco-root-evac-"+owner[:16], "haco-root-evac-"+owner[:16]
	generation, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	p := snapshotRootfsPlan{Pool: pool, Source: "haco-evac-source", SourceInstanceID: generation, Owner: owner}
	root, err := os.MkdirTemp("/var/lib", "haco-saved-rootfs-evacuation-")
	if err != nil {
		t.Fatal(err)
	}
	const key = "user.hacocoon.evacuation-test"
	// Reserve exact resource identities before creation. Failure keeps the ledger.
	plan, err := os.OpenFile(filepath.Join(root, "plan.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.NewEncoder(plan).Encode(map[string]any{"project": project, "pool": pool, "owner": owner, "source": p.Source, "saved": p.target(), "target": "destination", "generation": generation}); err != nil {
		t.Fatal(err)
	}
	if err = plan.Sync(); err != nil {
		t.Fatal(err)
	}
	if err = plan.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("retained fixture ledger and archive", root)
	run("incus", "storage", "create", pool, "btrfs", "size=1GiB", key+"="+owner)
	run("incus", "project", "create", project, "-c", "features.images=true", "-c", key+"="+owner)
	run("incus", "init", "--empty", p.Source, "--project", project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+generation, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue, "--config", key+"="+owner, "--config", "boot.autostart=false", "--config", "environment.OLD_TOKEN=synthetic-only")
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("saved-only rootfs without Base or Incus export\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("incus", "file", "push", marker, p.Source+"/root/retained", "--create-dirs", "--project", project)
	runtime := New(runner)
	runtime.project = project
	if err := runtime.createSnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	// The native copy has its exact saved ownership config; reserve/record before
	// any fallible verification just as the canonical snapshot coordinator does.
	receipt, err := os.OpenFile(filepath.Join(root, "saved-created.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.NewEncoder(receipt).Encode(p); err != nil {
		t.Fatal(err)
	}
	if err = receipt.Sync(); err != nil {
		t.Fatal(err)
	}
	if err = receipt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	if run("incus", "config", "get", p.Source, key, "--project", project) != owner {
		t.Fatal("foreign source")
	}
	run("incus", "delete", p.Source, "--project", project)
	if err := runtime.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal("saved copy depended on source", err)
	}
	path := func(instance string) string {
		t.Helper()
		result := filepath.Join("/var/lib/incus/storage-pools", pool, "containers", project+"_"+instance, "rootfs")
		resolved, err := filepath.EvalSymlinks(result)
		if err != nil || resolved != result {
			t.Fatal("native owned rootfs path unavailable or redirected", err)
		}
		return result
	}
	savedRoot := path(p.target())
	archive := filepath.Join(root, "saved-rootfs.tar")
	// No publish/export/snapshot operation takes place during capture. Nothing is
	// started; only this test's quiescent rootfs and known archive are used.
	run("env", "-u", "TAR_OPTIONS", "tar", "--one-file-system", "--acls", "--xattrs", "--numeric-owner", "--sparse", "-cpf", archive, "-C", savedRoot, ".")
	bytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(bytes)
	newGeneration, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	run("incus", "init", "--empty", "destination", "--project", project, "--no-profiles", "--storage", pool, "--config", key+"="+owner, "--config", environmentInstanceKey+"="+newGeneration, "--config", "boot.autostart=false", "--config", "security.privileged=false")
	targetRoot := path("destination")
	run("env", "-u", "TAR_OPTIONS", "tar", "--acls", "--xattrs", "--numeric-owner", "--same-owner", "--same-permissions", "-xpf", archive, "-C", targetRoot)
	run("env", "-u", "TAR_OPTIONS", "tar", "--acls", "--xattrs", "--numeric-owner", "-dpf", archive, "-C", targetRoot)
	readback := filepath.Join(root, "readback")
	run("incus", "file", "pull", "destination/root/retained", readback, "--project", project)
	got, err := os.ReadFile(readback)
	want, readErr := os.ReadFile(marker)
	if err != nil || readErr != nil || string(got) != string(want) {
		t.Fatal("saved-only bytes lost", err, readErr)
	}
	var observed snapshotInstanceObservation
	if err := json.Unmarshal([]byte(run("incus", "query", "/1.0/instances/destination?project="+project)), &observed); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(observed.Status, "stopped") || observed.Config[environmentInstanceKey] != newGeneration || observed.ExpandedConfig["environment.OLD_TOKEN"] != "" || len(observed.Profiles) != 0 || len(observed.ExpandedDevices) != 1 {
		t.Fatal("old authority or devices adopted")
	}
	// Alter only the destination, then verify the source saved object and archive.
	if err := os.WriteFile(filepath.Join(targetRoot, "root", "retained"), []byte("independent destination\n"), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(savedRoot, "root", "retained"))
	if err != nil || string(saved) != string(want) {
		t.Fatal("destination modified saved rootfs", err)
	}
	if err := runtime.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(archive)
	if err != nil || sha256.Sum256(after) != digest {
		t.Fatal("archive changed", err)
	}
	if run("incus", "config", "get", "destination", key, "--project", project) != owner {
		t.Fatal("foreign destination")
	}
	run("incus", "delete", "destination", "--project", project)
	if err := runtime.deleteSnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	var instances []any
	if err := json.Unmarshal([]byte(run("incus", "list", "--project", project, "--format=json")), &instances); err != nil || len(instances) != 0 {
		t.Fatal("owned instances not positively absent", err)
	}
	if run("incus", "project", "get", project, key) != owner {
		t.Fatal("foreign project")
	}
	run("incus", "project", "delete", project)
	if run("incus", "storage", "get", pool, key) != owner {
		t.Fatal("foreign pool")
	}
	var volumes []any
	if err := json.Unmarshal([]byte(run("incus", "storage", "volume", "list", pool, "--all-projects", "--format=json")), &volumes); err != nil || len(volumes) != 0 {
		t.Fatal("pool not empty", err)
	}
	run("incus", "storage", "delete", pool)
	var pools []struct{ Name string }
	if err := json.Unmarshal([]byte(run("incus", "storage", "list", "--format=json")), &pools); err != nil {
		t.Fatal(err)
	}
	for _, item := range pools {
		if item.Name == pool {
			t.Fatal("pool still present")
		}
	}
	t.Logf("saved rootfs direct evacuation passed; archive %s sha256 %x; stopped synthetic rootfs only, no Base/image/export/startup or full installation claim", archive, digest)
}
