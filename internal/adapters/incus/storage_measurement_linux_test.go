//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const storageMeasurementFileBytes = 8 << 20

type storagePoolMeasurement struct {
	t       *testing.T
	ctx     context.Context
	runtime *Runtime
	pool    string
	backing string
	before  os.FileInfo
	command func(string, ...string) string
}

// Capture the exact managed pool before any consumer is paused. This is an
// observation boundary, not authority to mutate or repair backing storage.
func storageMeasurementPool(t *testing.T, ctx context.Context, runtime *Runtime, name string, command func(string, ...string) string) storagePoolMeasurement {
	t.Helper()
	backing := filepath.Join("/var/lib/incus/disks", name+".img")
	var pool struct {
		Name, Driver string
		Config       map[string]string
	}
	if !safeIncusRef(name) || json.Unmarshal([]byte(command("incus", "query", "/1.0/storage-pools/"+name)), &pool) != nil || pool.Name != name || pool.Driver != "btrfs" || pool.Config["source"] != backing {
		t.Fatal("measurement pool identity unavailable")
	}
	if matches, known := runtime.inspectLiveStorage(ctx, name, backing); !known || !matches {
		t.Fatal("measurement pool mount identity or policy unavailable")
	}
	resolved, err := filepath.EvalSymlinks(backing)
	backingBefore, statErr := os.Lstat(backing)
	if err != nil || resolved != backing || statErr != nil || !backingBefore.Mode().IsRegular() {
		t.Fatal("measurement backing identity unavailable")
	}
	return storagePoolMeasurement{t: t, ctx: ctx, runtime: runtime, pool: name, backing: backing, before: backingBefore, command: command}
}

// Recheck the same backing file and live mount around the allocation read.
// Call before resuming consumers; a failed observation retains the fixture.
func (m storagePoolMeasurement) finish(phase, fixture string) {
	m.t.Helper()
	info, err := os.Lstat(m.backing)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(info, m.before) || info.Size() != m.before.Size() {
		m.t.Fatal("measurement pool backing identity or capacity changed")
	}
	allocation, err := parseStorageDU(m.command("du", "--summarize", "--block-size=1", "--", m.backing), m.backing)
	if err != nil {
		m.t.Fatal(err)
	}
	if matches, known := m.runtime.inspectLiveStorage(m.ctx, m.pool, m.backing); !known || !matches {
		m.t.Fatal("measurement pool identity changed")
	}
	backingAfter, err := os.Lstat(m.backing)
	if err != nil || !backingAfter.Mode().IsRegular() || !os.SameFile(backingAfter, m.before) || backingAfter.Size() != m.before.Size() {
		m.t.Fatal("measurement pool backing identity or capacity changed during reads")
	}
	// Other fixture rootfs/images and trusted Host activity use this pool. These
	// counters are contextual observations, never isolated workload attribution.
	m.t.Logf("storage_measurement fixture=%s phase=%s pool_scope=whole_shared_pool pool_logical_bytes=%d pool_allocated_bytes=%d", fixture, phase, info.Size(), allocation)
}

// All measured inputs are fixed fixture files. Refuse links, extra files and
// unexpected sizes before recursive counters; hash only bounded regular files.
func storageMeasurementHashes(path string) (result map[string]string, resultErr error) {
	if err := storageMeasurementDirectory(path); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	entries, err := directory.ReadDir(3)
	if err != nil && err != io.EOF || len(entries) > 2 {
		return nil, fmt.Errorf("unexpected measurement payload inventory")
	}
	hashes := map[string]string{}
	for _, entry := range entries {
		if entry.Name() != "base" && entry.Name() != "delta" {
			return nil, fmt.Errorf("unexpected measurement payload")
		}
		info, err := root.Lstat(entry.Name())
		if err != nil || !info.Mode().IsRegular() || info.Size() != storageMeasurementFileBytes {
			return nil, fmt.Errorf("invalid measurement payload type or size")
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 {
			return nil, fmt.Errorf("measurement payload must have one link")
		}
		file, err := root.Open(entry.Name())
		if err != nil {
			return nil, err
		}
		opened, statErr := file.Stat()
		digest := sha256.New()
		count, readErr := io.Copy(digest, io.LimitReader(file, storageMeasurementFileBytes+1))
		closeErr := file.Close()
		if statErr != nil || !os.SameFile(info, opened) || readErr != nil || closeErr != nil || count != storageMeasurementFileBytes {
			return nil, fmt.Errorf("measurement payload changed")
		}
		hashes[entry.Name()] = hex.EncodeToString(digest.Sum(nil))
	}
	return hashes, nil
}
