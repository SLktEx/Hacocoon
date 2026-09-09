//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRealIncusOwnedVolumeExportE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_VOLUME_TRANSFER") != "1" {
		t.Skip("requires dedicated root Incus/Btrfs with daemon storage mounts visible")
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
			t.Fatalf("native fixture failed %v: %v", args, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(nonce[:])
	pool := "haco-owned-export-" + owner[:16]
	p := snapshotVolumeFixture("work")
	p.Pool = pool
	p.Owner = owner
	runtime := New(runner)
	runtime.project = "default"
	c, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "default", Volume: &p})
	if err != nil {
		t.Fatal(err)
	}
	c.State = "verified"
	dir, err := os.MkdirTemp("/var/lib", "haco-owned-volume-export-")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.OpenFile(filepath.Join(dir, "plan.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(plan).Encode(map[string]any{"pool": pool, "owner": owner, "project": "default", "saved": c, "import": "restored"}); err != nil {
		t.Fatal(err)
	}
	if err := plan.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := plan.Close(); err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Sync(); err != nil {
		t.Fatal(err)
	}
	parent.Close()
	t.Log("exact isolated fixture plan; retain resources on failure", dir)
	const key = "user.hacocoon.transfer-test"
	run("storage", "create", pool, "btrfs", "size=1GiB", key+"="+owner)
	args := []string{"storage", "volume", "create", pool, p.target()}
	for k, v := range p.targetConfig() {
		args = append(args, k+"="+v)
	}
	run(args...)
	if err := runtime.verifySnapshotVolume(ctx, p); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", "default_"+p.target())
	info, err := os.Lstat(source)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("fixture must see daemon storage mounts", err)
	}
	marker := []byte("native saved volume through anonymous export\n")
	if err := os.WriteFile(filepath.Join(source, "retained"), marker, 0600); err != nil {
		t.Fatal(err)
	}
	archive, err := runtime.ExportSnapshotVolume(ctx, c, dir, 16<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	output := filepath.Join(dir, "volume.tar")
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(file, archive.Reader()); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	run("storage", "volume", "import", pool, output, "restored")
	if run("storage", "volume", "get", pool, "restored", "user.hacocoon.owner") != owner {
		t.Fatal("fixture imported ownership marker changed")
	}
	if err := runtime.deleteSnapshotVolume(ctx, p); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join("/var/lib/incus/storage-pools", pool, "custom", "default_restored", "retained"))
	if err != nil || string(restored) != string(marker) {
		t.Fatal("native archive lost data", err)
	}
	run("storage", "volume", "delete", pool, "restored")
	var remaining []any
	if json.Unmarshal([]byte(run("storage", "volume", "list", pool, "--all-projects", "--format=json")), &remaining) != nil || remaining == nil || len(remaining) != 0 {
		t.Fatal("pool not positively empty")
	}
	if run("storage", "get", pool, key) != owner {
		t.Fatal("foreign pool")
	}
	run("storage", "delete", pool)
	data, err := os.ReadFile(output)
	hash := sha256.Sum256(data)
	if err != nil || int64(len(data)) != archive.Size() || hex.EncodeToString(hash[:]) != archive.Digest() {
		t.Fatal("retained archive changed", err)
	}
	t.Logf("native owned volume export/import passed; source removed; archive %s bytes %d sha256 %s; no public aggregate import", output, archive.Size(), archive.Digest())
}
