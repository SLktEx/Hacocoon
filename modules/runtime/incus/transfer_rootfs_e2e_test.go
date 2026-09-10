//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRealIncusSnapshotRootfsExportE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_ROOTFS_TRANSFER") != "1" {
		t.Skip("requires dedicated root Incus/Btrfs acceptance")
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
			t.Fatalf("native fixture %v failed: %v", args, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(random[:])
	name := "haco-root-export-" + owner[:16]
	instanceID, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	p := snapshotRootfsPlan{Pool: name, Source: "haco-export-source", SourceInstanceID: instanceID, Owner: owner}
	root, err := os.MkdirTemp("/var/lib", "haco-rootfs-export-")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.OpenFile(filepath.Join(root, "plan.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(plan).Encode(map[string]string{"project": name, "pool": name, "source": p.target(), "owner": owner, "destination": "destination"}); err != nil {
		t.Fatal(err)
	}
	if err := plan.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := plan.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("exact isolated fixture and retained archive", root)
	const key = "user.hacocoon.transfer-test"
	run("storage", "create", name, "btrfs", "size=1GiB", key+"="+owner)
	run("project", "create", name, "-c", "features.images=true", "-c", key+"="+owner)
	args := []string{"init", "--empty", p.target(), "--project", name, "--no-profiles", "--storage", name}
	keys := []string{}
	for key := range p.config() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--config", key+"="+p.config()[key])
	}
	run(args...)
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("saved rootfs independent of Base and transport image\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("file", "push", marker, p.target()+"/root/retained", "--create-dirs", "--project", name)
	runtime := New(runner)
	runtime.project = name
	component, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: name, Rootfs: &p})
	if err != nil {
		t.Fatal(err)
	}
	component.State = "verified"
	archive, err := runtime.ExportSnapshotRootfs(ctx, component, root, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var images []any
	if err := json.Unmarshal([]byte(run("image", "list", "--format=json", "--project", name)), &images); err != nil || len(images) != 0 {
		t.Fatal("transport image cleanup not confirmed", err)
	}
	receipts, err := filepath.Glob(filepath.Join(root, "rootfs-export-*.jsonl"))
	if err != nil || len(receipts) != 0 {
		t.Fatal("successful export retained receipt", err)
	}
	if err := runtime.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal("saved source changed", err)
	}
	saved := filepath.Join(root, "rootfs.tar")
	output, err := os.OpenFile(saved, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(output, archive.Reader())
	if err != nil || n != archive.Size() {
		t.Fatal("archive copy", err)
	}
	if err := output.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	run("image", "import", saved, "--project", name)
	run("init", "local:"+archive.Digest(), "destination", "--project", name, "--no-profiles", "--storage", name, "--config", key+"="+owner, "--config", "security.privileged=false", "--config", "boot.autostart=false")
	readback := filepath.Join(root, "readback")
	run("file", "pull", "destination/root/retained", readback, "--project", name)
	raw, err := os.ReadFile(readback)
	if err != nil || string(raw) != "saved rootfs independent of Base and transport image\n" {
		t.Fatal("native archive lost rootfs", err)
	}
	if run("config", "get", "destination", key, "--project", name) != owner {
		t.Fatal("foreign destination")
	}
	run("delete", "destination", "--project", name)
	if err := runtime.deleteSnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	// The imported image is the exact fingerprint of this invocation's archive.
	run("image", "delete", archive.Digest(), "--project", name)
	if run("project", "get", name, key) != owner {
		t.Fatal("foreign project")
	}
	run("project", "delete", name)
	if run("storage", "get", name, key) != owner {
		t.Fatal("foreign pool")
	}
	var volumes []any
	if err := json.Unmarshal([]byte(run("storage", "volume", "list", name, "--all-projects", "--format=json")), &volumes); err != nil || len(volumes) != 0 {
		t.Fatal("pool not empty", err)
	}
	run("storage", "delete", name)
	t.Logf("owned rootfs export and native import passed; archive %s sha256 %s; no Base, public bundle or boot acceptance", saved, archive.Digest())
}
