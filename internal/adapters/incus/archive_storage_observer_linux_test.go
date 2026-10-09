//go:build linux

package incus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	environmenttransfer "github.com/SLktEx/Hacocoon/internal/env/transfer"
)

const archiveMeasurementTestLimit int64 = 4 << 30

func archiveMeasurementFixture(t *testing.T) ([]byte, environmenttransfer.Manifest) {
	t.Helper()
	manifest := environmenttransfer.Manifest{
		Version: 2, Source: "private-source-name", HasOCI: true,
		Workspaces: []environmenttransfer.Workspace{{Role: "workspace", Name: "private-repository-name", Remote: "https://github.com/private-owner/private-repository.git", Branch: "private-branch"}},
		Data:       []environmenttransfer.Data{{Key: "private-cache-key", Target: "/private-cache-target", Kind: "build-cache"}},
	}
	var parts []io.Reader
	for _, role := range []string{"rootfs", "workspace", "oci", "data-001"} {
		payload := []byte("opaque-private-native-" + role + "-payload")
		digest := sha256.Sum256(payload)
		manifest.Components = append(manifest.Components, environmenttransfer.Component{Role: role, Bytes: int64(len(payload)), SHA256: hex.EncodeToString(digest[:])})
		parts = append(parts, bytes.NewReader(payload))
	}
	var archive bytes.Buffer
	if err := environmenttransfer.Write(&archive, manifest, parts, archiveMeasurementTestLimit); err != nil {
		t.Fatal("could not construct valid archive fixture")
	}
	return archive.Bytes(), manifest
}

func archiveMeasurementPrivateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if os.Chmod(dir, 0700) != nil {
		t.Fatal("could not create private fixture parent")
	}
	return dir
}

func writeArchiveMeasurementFixture(t *testing.T) (string, []byte, environmenttransfer.Manifest) {
	t.Helper()
	data, manifest := archiveMeasurementFixture(t)
	path := filepath.Join(archiveMeasurementPrivateDir(t), "private.haco")
	if os.WriteFile(path, data, 0600) != nil {
		t.Fatal("could not write archive fixture")
	}
	return path, data, manifest
}

func TestArchiveStorageMeasurementVerifiesRetainedFileAndMinimizesJSON(t *testing.T) {
	path, data, wantManifest := writeArchiveMeasurementFixture(t)
	retained, manifest, sample, err := openArchiveStorageMeasurement(context.Background(), path, archiveMeasurementTestLimit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = retained.Close() })
	if !reflect.DeepEqual(manifest, wantManifest) {
		t.Fatal("private verified manifest lost transport metadata")
	}
	var stat syscall.Stat_t
	var filesystem syscall.Statfs_t
	if syscall.Stat(path, &stat) != nil || syscall.Fstatfs(int(retained.file.Fd()), &filesystem) != nil {
		t.Fatal("fixture stat unavailable")
	}
	digest := sha256.Sum256(data)
	var componentBytes uint64
	for i, component := range manifest.Components {
		componentBytes += uint64(component.Bytes)
		if sample.Components[i] != (archiveStorageComponent{Role: component.Role, Bytes: component.Bytes, SHA256: component.SHA256}) {
			t.Fatal("verified component projection differs")
		}
	}
	if sample.LogicalBytes != uint64(len(data)) || sample.AllocatedBytes != uint64(stat.Blocks)*512 || sample.SHA256 != hex.EncodeToString(digest[:]) || sample.ComponentBytes != componentBytes || sample.EnvelopeOverheadBytes != uint64(len(data))-componentBytes || sample.FilesystemType != int64(filesystem.Type) {
		t.Fatal("archive byte or filesystem observations differ from retained file")
	}
	for _, identity := range []string{sample.FileIdentitySHA256, sample.FilesystemIdentitySHA256} {
		decoded, err := hex.DecodeString(identity)
		if err != nil || len(decoded) != sha256.Size {
			t.Fatal("archive identity must be hashed")
		}
	}
	raw, err := json.Marshal(sample)
	if err != nil {
		t.Fatal("could not serialize archive sample")
	}
	for _, private := range []string{path, filepath.Dir(path), "private-", "opaque-private", "source", "workspaces", "remote", "branch", "target", "uid", "gid", "inode", "device"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("archive sample leaked private metadata or raw file identity")
		}
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object) != 9 {
		t.Fatal("archive sample schema changed")
	}
	for _, key := range []string{"logical_bytes", "allocated_bytes", "sha256", "filesystem_type", "filesystem_identity_sha256", "file_identity_sha256", "components", "component_bytes", "envelope_overhead_bytes"} {
		if _, ok := object[key]; !ok {
			t.Fatal("archive sample field missing")
		}
	}
	var components []map[string]json.RawMessage
	if json.Unmarshal(object["components"], &components) != nil || len(components) != 4 {
		t.Fatal("archive component samples missing")
	}
	for _, component := range components {
		if len(component) != 3 || component["role"] == nil || component["bytes"] == nil || component["sha256"] == nil {
			t.Fatal("component samples must contain role, bytes and digest only")
		}
	}
	// Callers can use the private manifest without changing future measurements.
	manifest.Components[0].SHA256 = "caller-mutated"
	for range 3 {
		again, err := retained.Observe(context.Background())
		if err != nil || !reflect.DeepEqual(sample, again) {
			t.Fatal("unchanged retained archive observation changed")
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatal("observer must not extract or copy the archive")
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := retained.Observe(context.Background()); err == nil {
		t.Fatal("closed archive observer remained usable")
	}
}

func TestArchiveStorageMeasurementRejectsInvalidBundle(t *testing.T) {
	good, _ := archiveMeasurementFixture(t)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"manifest", bytes.Replace(good, []byte(`"version":2`), []byte(`"version":9`), 1)},
		{"payload", bytes.Replace(good, []byte("opaque-private-native-rootfs-payload"), []byte("changed-private-native-rootfs-data!"), 1)},
		{"closing_blocks", good[:len(good)-512]},
		{"trailing_bytes", append(append([]byte(nil), good...), 0)},
		{"empty", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(archiveMeasurementPrivateDir(t), "private-invalid.haco")
			if os.WriteFile(path, tc.data, 0600) != nil {
				t.Fatal("could not write invalid fixture")
			}
			retained, manifest, sample, err := openArchiveStorageMeasurement(context.Background(), path, archiveMeasurementTestLimit)
			if err == nil || retained != nil || !reflect.DeepEqual(manifest, environmenttransfer.Manifest{}) || !reflect.DeepEqual(sample, archiveStorageSample{}) {
				t.Fatal("invalid archive produced a verified observation")
			}
			if strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), filepath.Dir(path)) {
				t.Fatal("verification error leaked input or path")
			}
		})
	}
}

func TestArchiveStorageMeasurementRefusesUnsafeFileOrParent(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory", "fifo", "writable_file", "writable_parent", "readable_parent", "parent_symlink", "writable_ancestor", "ancestor_symlink", "noncanonical"} {
		t.Run(kind, func(t *testing.T) {
			path, _, _ := writeArchiveMeasurementFixture(t)
			switch kind {
			case "symlink":
				target := path + ".original"
				if os.Rename(path, target) != nil || os.Symlink(target, path) != nil {
					t.Fatal("could not create symlink fixture")
				}
			case "hardlink":
				if os.Link(path, path+".link") != nil {
					t.Fatal("could not create hardlink fixture")
				}
			case "directory", "fifo":
				if os.Remove(path) != nil {
					t.Fatal("could not replace fixture")
				}
				if kind == "directory" && os.Mkdir(path, 0700) != nil || kind == "fifo" && syscall.Mkfifo(path, 0600) != nil {
					t.Fatal("could not create special-file fixture")
				}
			case "writable_file":
				if os.Chmod(path, 0620) != nil {
					t.Fatal("could not change fixture mode")
				}
			case "writable_parent", "readable_parent":
				mode := os.FileMode(0770)
				if kind == "readable_parent" {
					mode = 0750
				}
				if os.Chmod(filepath.Dir(path), mode) != nil {
					t.Fatal("could not change parent mode")
				}
			case "parent_symlink", "ancestor_symlink":
				link := filepath.Join(t.TempDir(), "private-link")
				target, suffix := filepath.Dir(path), filepath.Base(path)
				if kind == "ancestor_symlink" {
					target = filepath.Dir(target)
					suffix = filepath.Join(filepath.Base(filepath.Dir(path)), suffix)
				}
				if os.Symlink(target, link) != nil {
					t.Fatal("could not create parent symlink")
				}
				path = filepath.Join(link, suffix)
			case "writable_ancestor":
				ancestor := filepath.Dir(path)
				child := filepath.Join(ancestor, "private-child")
				if os.Mkdir(child, 0700) != nil || os.Rename(path, filepath.Join(child, filepath.Base(path))) != nil || os.Chmod(ancestor, 0770) != nil {
					t.Fatal("could not create unsafe ancestor")
				}
				path = filepath.Join(child, filepath.Base(path))
			case "noncanonical":
				path = filepath.Dir(path) + "/../" + filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
			}
			retained, _, _, err := openArchiveStorageMeasurement(context.Background(), path, archiveMeasurementTestLimit)
			if err == nil {
				_ = retained.Close()
				t.Fatal("unsafe archive observation accepted")
			}
			if strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), filepath.Dir(path)) {
				t.Fatal("unsafe-file error leaked private path")
			}
		})
	}
}

func TestArchiveStorageMeasurementRejectsChangedRetainedIdentityOrBytes(t *testing.T) {
	for _, kind := range []string{"replacement", "content", "valid_metadata", "size", "removed", "mode", "new_hardlink", "parent_replacement", "parent_mode"} {
		t.Run(kind, func(t *testing.T) {
			path, data, _ := writeArchiveMeasurementFixture(t)
			retained, _, _, err := openArchiveStorageMeasurement(context.Background(), path, archiveMeasurementTestLimit)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := retained.Close(); err != nil {
					t.Error(err)
				}
			}()
			switch kind {
			case "replacement":
				if os.WriteFile(path+".new", data, 0600) != nil || os.Rename(path+".new", path) != nil {
					t.Fatal("could not replace retained file")
				}
			case "content", "valid_metadata":
				if kind == "content" {
					data = bytes.Replace(data, []byte("opaque-private"), []byte("edited-private"), 1)
				} else {
					// The bundle remains fully valid. Its envelope hash must still
					// detect changed metadata that component digests cannot cover.
					data = bytes.Replace(data, []byte("private-source-name"), []byte("changed-source-name"), 1)
					if _, err := environmenttransfer.Inspect(bytes.NewReader(data), archiveMeasurementTestLimit); err != nil {
						t.Fatal("metadata mutation fixture must remain valid")
					}
				}
				if os.WriteFile(path, data, 0600) != nil {
					t.Fatal("could not mutate retained file")
				}
			case "size":
				if os.Truncate(path, int64(len(data)-1)) != nil {
					t.Fatal("could not truncate retained file")
				}
			case "removed":
				if os.Remove(path) != nil {
					t.Fatal("could not remove fixture binding")
				}
			case "mode":
				if os.Chmod(path, 0400) != nil {
					t.Fatal("could not change file mode")
				}
			case "new_hardlink":
				if os.Link(path, path+".link") != nil {
					t.Fatal("could not change link count")
				}
			case "parent_replacement":
				parent := filepath.Dir(path)
				moved := parent + "-moved"
				if os.Rename(parent, moved) != nil || os.Mkdir(parent, 0700) != nil || os.Rename(filepath.Join(moved, filepath.Base(path)), path) != nil || os.Remove(moved) != nil {
					t.Fatal("could not replace parent around original inode")
				}
			case "parent_mode":
				if os.Chmod(filepath.Dir(path), 0755) != nil {
					t.Fatal("could not change parent permissions")
				}
			}
			if sample, err := retained.Observe(context.Background()); err == nil || !reflect.DeepEqual(sample, archiveStorageSample{}) {
				t.Fatal("changed retained archive produced an observation")
			}
		})
	}
}

func TestArchiveStorageMeasurementBudgetsAndCancellation(t *testing.T) {
	path, _, _ := writeArchiveMeasurementFixture(t)
	for _, limit := range []int64{-1, 0, 1, math.MaxInt64} {
		retained, _, _, err := openArchiveStorageMeasurement(context.Background(), path, limit)
		if err == nil {
			_ = retained.Close()
			t.Fatal("invalid or insufficient component budget accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := openArchiveStorageMeasurement(ctx, path, archiveMeasurementTestLimit); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not preserved")
	}
	//nolint:staticcheck // Deliberately exercise refusal of an invalid nil context.
	if _, _, _, err := openArchiveStorageMeasurement(nil, path, archiveMeasurementTestLimit); err == nil {
		t.Fatal("nil context accepted")
	}
	retained, _, _, err := openArchiveStorageMeasurement(context.Background(), path, archiveMeasurementTestLimit)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := retained.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := retained.Observe(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("reobservation ignored cancellation")
	}
	reader := archiveMeasurementReader{ctx: context.Background(), src: strings.NewReader(strings.Repeat("x", 128<<10))}
	if n, err := reader.Read(make([]byte, 128<<10)); err != nil || n != 64<<10 {
		t.Fatal("archive read chunk is not bounded")
	}
	reader.ctx = ctx
	if n, err := reader.Read(make([]byte, 1)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("archive reader ignored cancellation")
	}
}

func TestArchiveStorageMeasurementCounterAndOwnershipChecks(t *testing.T) {
	_, manifest := archiveMeasurementFixture(t)
	stat := syscall.Stat_t{Dev: 123, Ino: 456, Size: 4096, Blocks: 2, Mode: syscall.S_IFREG | 0600, Nlink: 1, Uid: uint32(os.Geteuid()), Gid: uint32(os.Getegid())}
	filesystem := syscall.Statfs_t{Type: 0x9123683E}
	sample, err := archiveMeasurementSample(stat, filesystem, manifest.Components, strings.Repeat("a", 64))
	if err != nil || sample.AllocatedBytes != 1024 || sample.LogicalBytes != 4096 {
		t.Fatal("referenced block accounting must use 512-byte units")
	}
	more := stat
	more.Blocks = 3
	other, err := archiveMeasurementSample(more, filesystem, manifest.Components, strings.Repeat("a", 64))
	if err != nil || other.AllocatedBytes != 1536 || other.FileIdentitySHA256 != sample.FileIdentitySHA256 || !sameArchiveMeasurementFile(stat, more) {
		t.Fatal("allocation change must remain a separate observation")
	}
	for _, change := range []func(*syscall.Stat_t){
		func(s *syscall.Stat_t) { s.Blocks = -1 },
		func(s *syscall.Stat_t) { s.Blocks = math.MaxInt64 },
		func(s *syscall.Stat_t) { s.Size = -1 },
		func(s *syscall.Stat_t) { s.Size = 1 },
		func(s *syscall.Stat_t) { s.Size = archiveMeasurementEnvelopeBudget + 4096 },
	} {
		bad := stat
		change(&bad)
		if _, err := archiveMeasurementSample(bad, filesystem, manifest.Components, strings.Repeat("a", 64)); err == nil {
			t.Fatal("invalid archive counters accepted")
		}
	}
	for _, change := range []func(*syscall.Stat_t){
		func(s *syscall.Stat_t) { s.Uid++ },
		func(s *syscall.Stat_t) { s.Gid++ },
		func(s *syscall.Stat_t) { s.Ino++ },
		func(s *syscall.Stat_t) { s.Dev++ },
		func(s *syscall.Stat_t) { s.Nlink++ },
		func(s *syscall.Stat_t) { s.Mode ^= 0400 },
	} {
		bad := stat
		change(&bad)
		if sameArchiveMeasurementFile(stat, bad) {
			t.Fatal("changed identity or ownership accepted")
		}
	}
	// Socket creation is unnecessary for checking the observer's regular-file
	// boundary, and unavailable in network-restricted test environments.
	for _, kind := range []uint32{syscall.S_IFSOCK, syscall.S_IFCHR, syscall.S_IFBLK, syscall.S_IFIFO, syscall.S_IFDIR, syscall.S_IFLNK} {
		special := stat
		special.Mode = kind | 0600
		if safeArchiveMeasurementRegular(special) {
			t.Fatal("special-file mode accepted")
		}
	}
	foreign := stat
	foreign.Uid++
	if safeArchiveMeasurementRegular(foreign) {
		t.Fatal("foreign file ownership accepted")
	}
	for _, components := range [][]environmenttransfer.Component{
		nil,
		{{Role: "rootfs", Bytes: -1}, {Role: "workspace", Bytes: 1}},
		{{Role: "rootfs", Bytes: math.MaxInt64}, {Role: "workspace", Bytes: 1}},
		{{Role: "rootfs", Bytes: 2048}, {Role: "workspace", Bytes: 2048}},
		{{Role: "rootfs", Bytes: 4090}, {Role: "workspace", Bytes: math.MaxInt64}},
	} {
		if _, err := archiveMeasurementSample(stat, filesystem, components, strings.Repeat("a", 64)); err == nil {
			t.Fatal("invalid component accounting accepted")
		}
	}
}
