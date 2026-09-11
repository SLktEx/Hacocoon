//go:build linux && (amd64 || arm64)

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"golang.org/x/sys/unix"
)

func TestReclaimBackingHandlePreservesIdentityAndAllocation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pool.img")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(16 << 20); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("retained"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	held, err := openReclaimPath(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	target := &pinnedReclaimTarget{backing: held}
	before, err := target.Allocation()
	if err != nil {
		t.Fatal(err)
	}
	if before.LogicalBytes != 16<<20 || before.AllocatedBytes == 0 || before.AllocatedBytes >= before.LogicalBytes {
		t.Fatalf("sparse allocation not distinguished: %+v", before)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := target.Allocation()
	if err != nil || after != before {
		t.Fatalf("handle followed replaced path: %+v %v", after, err)
	}
	if err := os.Link(path+".old", path+".link"); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Allocation(); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("hardlink accepted: %v", err)
	}
}

func TestReclaimTargetRefusesSymlinksAndNonBtrfs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "disks"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "storage-pools", "pool"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "disks", "pool.img")
	if err := os.WriteFile(source, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if target, err := pinReclaimTarget("pool", source); err == nil {
		target.Close()
		t.Fatal("non-Btrfs target accepted")
	}
	link := root + "-link"
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)
	if f, err := openReclaimPath(filepath.Join(link, "disks", "pool.img"), false); err == nil {
		f.Close()
		t.Fatal("symlink ancestor accepted")
	}
	if target, err := pinReclaimTarget("--option", source); err == nil {
		target.Close()
		t.Fatal("invalid pool accepted")
	}
}

func TestRealIncusReclaimTargetInspection(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_INSPECT") != "1" {
		t.Skip("opt-in read-only Incus/Btrfs handle verification")
	}
	pool, source := os.Getenv("HACO_E2E_RECLAIM_POOL"), os.Getenv("HACO_E2E_RECLAIM_SOURCE")
	target, err := pinReclaimTarget(pool, source)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	before, err := target.Allocation()
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Validate(); err != nil {
		t.Fatal(err)
	}
	t.Logf("PASS exact Btrfs/loop/backing handles; logical=%d allocated=%d; no trim or resize executed", before.LogicalBytes, before.AllocatedBytes)
}

func TestRealIncusMountedReclaimInspection(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_MOUNTED") != "1" {
		t.Skip("opt-in isolated instance with real cached image/Btrfs pool")
	}
	pool, source, image := os.Getenv("HACO_E2E_RECLAIM_POOL"), os.Getenv("HACO_E2E_RECLAIM_SOURCE"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !diagnosticPoolName.MatchString(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root, pool and full image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	r := New(host.ExecRunner{})
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	ref := "haco-reclaim-" + hex.EncodeToString(random[:])
	generation, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("/var/lib", "haco-reclaim-inspect-")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct{ Project, Instance, Generation, Pool, Source string }{r.project, ref, generation, pool, source})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.OpenFile(filepath.Join(dir, "ownership.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned fixture %s; failure retains receipt and instance: %s", ref, dir)
	command := func(args ...string) {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
			t.Fatal("fixture Incus operation failed", err)
		}
	}
	command("init", "local:"+image, ref, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+generation, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue)
	command("start", ref, "--project", r.project)
	target, err := pinReclaimTarget(pool, source)
	if err != nil {
		t.Fatal(err)
	}
	before, err := target.Allocation()
	if err != nil {
		_ = target.Close()
		t.Fatal(err)
	}
	if err := target.Validate(); err != nil {
		_ = target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.VerifyEnvironmentIdentity(ctx, ref, generation); err != nil {
		t.Fatal(err)
	}
	command("delete", ref, "--project", r.project, "--force")
	if exists, err := r.environmentExists(ctx, ref); err != nil || exists {
		t.Fatal("fixture absence unproven", err)
	}
	t.Logf("PASS mounted Btrfs/loop/backing identity; logical=%d allocated=%d; exact fixture cleanup; no trim/resize; shared image/pool and receipt retained", before.LogicalBytes, before.AllocatedBytes)
}

func TestTrimCanceledOrInvalidTargetNeverAttemptsMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := (&pinnedReclaimTarget{}).Trim(ctx)
	if !errors.Is(err, context.Canceled) || result.Attempted {
		t.Fatal(result, err)
	}
	file, err := os.CreateTemp(t.TempDir(), "regular")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("retained"); err != nil {
		t.Fatal(err)
	}
	target := &pinnedReclaimTarget{backing: file, mount: file, loop: file}
	result, err = target.Trim(context.Background())
	if err == nil || result.Attempted {
		t.Fatal("non-block target attempted trim", result, err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil || string(data) != "retained" {
		t.Fatal("invalid target changed", err)
	}
}

func TestRealIncusPoolTrimPreservesVolumeAndSnapshot(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_TRIM") != "1" {
		t.Skip("opt-in dedicated root Incus/Btrfs fixture with independent pool")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r := New(host.ExecRunner{})
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(random[:])
	pool := "haco-trim-" + owner[:16]
	volume := "haco-persistent-" + owner
	resource := core.PersistentResource{ID: "oci:trim-" + owner[:16], Owner: owner, Kind: OCIStoreKind, NativeRef: pool + "/" + volume, State: "ready", CreatedAt: time.Now().UTC()}
	dir, err := os.MkdirTemp("/var/lib", "haco-trim-")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Pool, Owner, Project string
		Resource             core.PersistentResource
	}{pool, owner, r.project, resource})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.OpenFile(filepath.Join(dir, "ownership.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned pool %s volume %s; failure retains %s", pool, volume, dir)
	command := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated {
			t.Fatal("fixture Incus operation failed", err)
		}
		return out.Stdout
	}
	command("storage", "create", pool, "btrfs", "size=1GiB", "btrfs.mount_options=compress=zstd:3,noatime,nodiscard", "user.hacocoon.test-owner="+owner)
	backend := &PersistentResourceBackend{Runtime: r}
	if err := backend.Create(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if err := backend.Verify(ctx, resource); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", r.project+"_"+volume)
	retained := []byte("retained Workspace/OCI bytes before trim")
	if err := os.WriteFile(filepath.Join(root, "retained"), retained, 0600); err != nil {
		t.Fatal(err)
	}
	command("storage", "volume", "snapshot", "create", pool, volume, "keep", "--project", r.project)
	filler := filepath.Join(root, "discardable")
	file, err := os.OpenFile(filler, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(file, rand.Reader, 64<<20); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	selected, err := r.PrepareStorageReclamation(ctx, BtrfsLoopPoolSpec{Name: pool, MountOptions: "compress=zstd:3,noatime,nodiscard"})
	if err != nil {
		t.Fatal(err)
	}
	target := selected.target
	// Flush test allocation before measuring; remove only the fixture filler.
	if err := unix.Syncfs(int(target.mount.Fd())); err != nil {
		_ = selected.Close()
		t.Fatal(err)
	}
	allocated, err := target.Allocation()
	if err != nil {
		_ = selected.Close()
		t.Fatal(err)
	}
	if err := backend.Verify(ctx, resource); err != nil {
		_ = selected.Close()
		t.Fatal(err)
	}
	if err := os.Remove(filler); err != nil {
		_ = selected.Close()
		t.Fatal(err)
	}
	result, trimErr := selected.TrimPool(ctx)
	if trimErr == nil && os.Getenv("HACO_E2E_RECLAIM_OUTER_TRIM") == "1" {
		// This extra opt-in authorizes the dedicated WSL fixture's outer filesystem,
		// not merely the new Incus pool. No mount/resize/delete is performed here.
		outer, outerErr := selected.TrimBackingFilesystem(ctx)
		if outerErr != nil || !outer.Attempted || !outer.KernelReportKnown {
			_ = selected.Close()
			t.Fatal("outer discard failed; this does not undo inner trim", outer, outerErr)
		}
		t.Logf("PASS outer ext4 discard via pinned image fd; kernel-reported=%d; Windows allocation not measured", outer.KernelTrimmedBytes)
	} else if trimErr == nil {
		t.Log("SKIP outer filesystem trim: separate dedicated-distribution opt-in not enabled")
	}
	closeErr := selected.Close()
	if trimErr != nil || closeErr != nil {
		t.Fatal(result, trimErr, closeErr)
	}
	if !result.Attempted || !result.KernelReportKnown || result.After.LogicalBytes != allocated.LogicalBytes || result.After.AllocatedBytes >= allocated.AllocatedBytes {
		t.Fatalf("physical reduction unproven: allocated=%+v trim=%+v", allocated, result)
	}
	current, err := os.ReadFile(filepath.Join(root, "retained"))
	if err != nil || string(current) != string(retained) {
		t.Fatal("volume bytes changed", err)
	}
	savedPath := filepath.Join("/var/lib/incus/storage-pools", pool, "custom-snapshots", r.project+"_"+volume, "keep", "retained")
	saved, err := os.ReadFile(savedPath)
	if err != nil || string(saved) != string(retained) {
		t.Fatal("saved bytes changed", err)
	}
	if err := backend.Verify(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(command("storage", "get", pool, "user.hacocoon.test-owner")) != owner {
		t.Fatal("pool ownership changed")
	}
	command("storage", "volume", "snapshot", "delete", pool, volume, "keep", "--project", r.project)
	if err := backend.Delete(ctx, resource); err != nil {
		t.Fatal(err)
	}
	command("storage", "delete", pool)
	t.Logf("PASS volume/snapshot bytes preserved; backing logical=%d allocated before filler removal=%d after trim=%d kernel-reported trim=%d; exact fixture cleanup; no pool resize or Windows compaction", allocated.LogicalBytes, allocated.AllocatedBytes, result.After.AllocatedBytes, result.KernelTrimmedBytes)
}

func TestOuterTrimCanceledOrInvalidTargetNeverAttemptsMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var target *pinnedReclaimTarget
	result, err := target.TrimBackingFilesystem(ctx)
	if !errors.Is(err, context.Canceled) || result.Attempted {
		t.Fatal(result, err)
	}
	result, err = target.TrimBackingFilesystem(context.Background())
	if !errors.Is(err, core.ErrInvalidArgument) || result.Attempted {
		t.Fatal(result, err)
	}
	f, err := os.CreateTemp(t.TempDir(), "outer-invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString("retained"); err != nil {
		t.Fatal(err)
	}
	target = &pinnedReclaimTarget{backing: f, mount: f, loop: f}
	result, err = target.TrimBackingFilesystem(context.Background())
	if err == nil || result.Attempted {
		t.Fatal("invalid native identity allowed discard", result, err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil || string(data) != "retained" {
		t.Fatal("invalid target changed", err)
	}
}

func TestReclaimFilesystemCountersDistinguishUseFromAllocation(t *testing.T) {
	valid := unix.Statfs_t{Type: unix.EXT4_SUPER_MAGIC, Bsize: 4096, Blocks: 100, Bfree: 30}
	got, err := reclaimFilesystemCounters(valid, unix.EXT4_SUPER_MAGIC)
	if err != nil || got.CapacityBytes != 409600 || got.UsedBytes != 286720 {
		t.Fatal(got, err)
	}
	for _, mutate := range []func(*unix.Statfs_t){
		func(f *unix.Statfs_t) { f.Type = unix.BTRFS_SUPER_MAGIC },
		func(f *unix.Statfs_t) { f.Bsize = 0 },
		func(f *unix.Statfs_t) { f.Bsize = -1 },
		func(f *unix.Statfs_t) { f.Blocks = 0 },
		func(f *unix.Statfs_t) { f.Bfree = 101 },
		func(f *unix.Statfs_t) { f.Blocks = ^uint64(0) },
	} {
		fs := valid
		mutate(&fs)
		if _, err := reclaimFilesystemCounters(fs, unix.EXT4_SUPER_MAGIC); err == nil {
			t.Fatal("invalid filesystem counters accepted", fs)
		}
	}
}
