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

func TestRealIncusSnapshotBaseE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SNAPSHOT_BASE") != "1" {
		t.Skip("set HACO_E2E_SNAPSHOT_BASE=1 on a dedicated root Incus/Btrfs host with cached image/pool")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root and explicit pool/full image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := New(host.ExecRunner{})
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	p := snapshotBasePlan{Pool: pool, Owner: hex.EncodeToString(nonce[:]), Base: core.BaseRef{Name: "snapshot-fixture/base", Revision: core.BaseRevision("sha256:" + image)}}
	file, err := os.CreateTemp("/var/lib", "haco-snapshot-base-*.json")
	if err != nil {
		t.Fatal(err)
	}
	component, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Base: &p})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(component)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned Base target %s; failure recovery record %s", p.target(), file.Name())
	savedPlan, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	var restored core.SnapshotComponent
	if err := json.Unmarshal(savedPlan, &restored); err != nil {
		t.Fatal(err)
	}
	source := core.SnapshotSource{Environment: core.Environment{Base: &p.Base}}
	if err := r.createSnapshotComponent(ctx, source, restored); err != nil {
		t.Fatal(err)
	}
	restored.State = "created"
	if err := r.verifySnapshotComponent(ctx, restored); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("/var/lib/incus/storage-pools", pool, "containers", r.project+"_"+p.target())
	original := filepath.Join("/var/lib/incus/storage-pools", pool, "images", image)
	field := func(path, key string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "btrfs", "subvolume", "show", path)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("Btrfs observation failed", err)
		}
		for _, line := range strings.Split(out.Stdout, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		return ""
	}
	uuid := field(original, "UUID")
	if uuid == "" || uuid == "-" || field(root, "Parent UUID") != uuid || field(root, "UUID") == uuid {
		t.Fatal("Base COW ancestry unproven")
	}
	marker := filepath.Join(root, "rootfs", "root", "snapshot-base-marker")
	info, err := os.Lstat(filepath.Dir(marker))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("owned saved root unavailable", err)
	}
	if err := os.WriteFile(marker, []byte("saved-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(original, "rootfs", "root", "snapshot-base-marker")); !os.IsNotExist(err) {
		t.Fatal("saved write affected cached image", err)
	}
	if err := r.deleteSnapshotComponent(ctx, restored); err != nil {
		t.Fatal(err)
	}
	if field(original, "UUID") != uuid {
		t.Fatal("deleting saved Base changed cached image")
	}
	if err := os.Remove(file.Name()); err != nil {
		t.Fatal(err)
	}
	t.Logf("PASS reloaded durable binding, exact Base revision, isolated stopped config, Btrfs parent UUID %s, independent saved write and owned cleanup; shared cached image retained", uuid)
}
