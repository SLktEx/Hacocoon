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

func TestRealIncusSnapshotRootfsE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SNAPSHOT_ROOTFS") != "1" {
		t.Skip("set HACO_E2E_SNAPSHOT_ROOTFS=1 on a dedicated root Incus/Btrfs host with cached image/pool")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root and explicit pool/full image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := New(host.ExecRunner{})
	command := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("Incus %v failed: %v", args, err)
		}
		return out.Stdout
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(nonce[:])
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	p := snapshotRootfsPlan{Pool: pool, Source: "haco-root-probe-" + owner[:16], SourceInstanceID: id, Owner: owner}
	stateDir, err := os.MkdirTemp("/var/lib", "haco-snapshot-root-")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(stateDir, "plan.json")
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(planPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
	t.Logf("owned source %s target %s; failure recovery record %s", p.Source, p.target(), planPath)
	command("init", "local:"+image, p.Source, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+id, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue, "--config", "environment.TEST_TOKEN=synthetic-fixture-token", "--config", "boot.autostart=true")
	command("config", "device", "add", p.Source, "fixture-workspace", "disk", "source="+stateDir, "path=/workspace", "--project", r.project)
	volumePath := func(name string) string {
		return filepath.Join("/var/lib/incus/storage-pools", pool, "containers", r.project+"_"+name)
	}
	original, saved := volumePath(p.Source), volumePath(p.target())
	root := filepath.Join(original, "rootfs", "root")
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("owned source root unavailable", err)
	}
	if err := os.WriteFile(filepath.Join(root, "snapshot-marker"), []byte("guest-only bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.createSnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := r.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	read := func(base string) string {
		t.Helper()
		value, err := os.ReadFile(filepath.Join(base, "rootfs", "root", "snapshot-marker"))
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	if read(saved) != "guest-only bytes" {
		t.Fatal("guest rootfs lost")
	}
	btrfs := func(path string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "btrfs", "subvolume", "show", path)
		if err != nil || out.StdoutTruncated {
			t.Fatal("Btrfs inspection failed", err)
		}
		return out.Stdout
	}
	field := func(output, key string) string {
		for _, line := range strings.Split(output, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		return ""
	}
	uuid := field(btrfs(original), "UUID")
	copyInfo := btrfs(saved)
	if uuid == "" || uuid == "-" || field(copyInfo, "Parent UUID") != uuid || field(copyInfo, "UUID") == uuid {
		t.Fatalf("rootfs COW ancestry unproven: source %s copy %s", uuid, copyInfo)
	}
	if err := os.WriteFile(filepath.Join(root, "snapshot-marker"), []byte("source edited"), 0600); err != nil {
		t.Fatal(err)
	}
	if read(saved) != "guest-only bytes" {
		t.Fatal("source edit affected save")
	}
	if err := r.VerifyEnvironmentIdentity(ctx, p.Source, id); err != nil {
		t.Fatal(err)
	}
	command("delete", p.Source, "--project", r.project)
	if exists, err := r.environmentExists(ctx, p.Source); err != nil || exists {
		t.Fatal("source absence unproven", err)
	}
	if read(saved) != "guest-only bytes" {
		t.Fatal("source deletion lost save")
	}
	if err := r.verifySnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := r.deleteSnapshotRootfs(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stateDir); err != nil {
		t.Fatal(err)
	}
	t.Logf("PASS rootfs COW parent UUID %s; no inherited token/device/profile/autostart; source edit/deletion independence; owned cleanup", uuid)
}
