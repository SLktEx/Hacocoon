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
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/sys/unix"
)

// NativeArchive owns complete native bytes, not source or destination authority.
// Only a read-only descriptor remains; no pathname is published to consumers.
type NativeArchive struct {
	file   *os.File
	size   int64
	digest string
}

func (a *NativeArchive) Size() int64    { return a.size }
func (a *NativeArchive) Digest() string { return a.digest }
func (a *NativeArchive) Reader() io.Reader {
	return &archiveReader{io.NewSectionReader(a.file, 0, a.size)}
}
func (a *NativeArchive) Close() error { return a.file.Close() }

type archiveReader struct{ reader io.Reader }

func (r *archiveReader) Read(p []byte) (int, error) { return r.reader.Read(p) }

type archiveContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *archiveContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type volumeBackupObservation struct {
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	VolumeOnly       bool      `json:"volume_only"`
	OptimizedStorage bool      `json:"optimized_storage"`
}

func (r *Runtime) savedVolumeBackups(ctx context.Context, p snapshotVolumePlan) (map[string]volumeBackupObservation, error) {
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool+"/volumes/custom/"+p.target()+"/backups?project="+r.project+"&recursion=1")
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, errors.Join(core.ErrRuntimeUnavailable, err)
	}
	var items []volumeBackupObservation
	if json.Unmarshal([]byte(out.Stdout), &items) != nil || items == nil || len(items) > 2048 {
		return nil, core.ErrIncompatibleState
	}
	result := make(map[string]volumeBackupObservation, len(items))
	for _, v := range items {
		if !safeIncusRef(v.Name) || v.CreatedAt.IsZero() {
			return nil, core.ErrIncompatibleState
		}
		if _, exists := result[v.Name]; exists {
			return nil, core.ErrIncompatibleState
		}
		result[v.Name] = v
	}
	return result, nil
}

// ExportSnapshotVolume uses ordinary native Incus archives. The caller must hold
// ReadSnapshot's source-use boundary through completion. root is a private local
// controller directory, never a guest path. No existing backup is deleted here.
func (r *Runtime) ExportSnapshotVolume(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (*NativeArchive, error) {
	b, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return nil, err
	}
	if b.Volume == nil || c.State != "verified" {
		return nil, core.ErrIncompatibleState
	}
	p := *b.Volume
	if err := r.verifySnapshotVolume(ctx, p); err != nil {
		return nil, err
	}
	before, err := r.savedVolumeBackups(ctx, p)
	if err != nil {
		return nil, err
	}
	return captureNativeArchive(ctx, root, limit, func(output string) error {
		out, err := r.runner.Run(ctx, "incus", "storage", "volume", "export", p.Pool, p.target(), output, "--project", r.project, "--volume-only", "--compression=none", "--quiet")
		if err != nil || out.ExitCode != 0 {
			return fmt.Errorf("Incus export failed for %s/%s/%s; inspect native backups: %w", r.project, p.Pool, p.target(), errors.Join(core.ErrRuntimeUnavailable, err))
		}
		if err := r.verifySnapshotVolume(ctx, p); err != nil {
			return err
		}
		after, err := r.savedVolumeBackups(ctx, p)
		if err != nil {
			return fmt.Errorf("Incus backup cleanup unconfirmed for %s/%s/%s: %w", r.project, p.Pool, p.target(), errors.Join(core.ErrRecoveryRequired, err))
		}
		for name, observed := range after {
			previous, existed := before[name]
			if !existed || !sameBackupObservation(previous, observed) {
				return fmt.Errorf("Incus backup cleanup unconfirmed for %s/%s/%s: %w", r.project, p.Pool, p.target(), core.ErrRecoveryRequired)
			}
		}
		return nil
	})
}
func sameBackupObservation(a, b volumeBackupObservation) bool {
	return a.Name == b.Name && a.CreatedAt.Equal(b.CreatedAt) && a.ExpiresAt.Equal(b.ExpiresAt) && a.VolumeOnly == b.VolumeOnly && a.OptimizedStorage == b.OptimizedStorage
}

// Incus 6.0.5 volume export opens its exact target with os.Create. A live parent
// proc-fd path lets that child write an unnamed file without stdout capture or a
// named residue. This does not seal data against privileged Host fd inspection.
func captureNativeArchive(ctx context.Context, root string, limit int64, produce func(string) error) (result *NativeArchive, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || !filepath.IsAbs(root) || filepath.Clean(root) != root || len(root) > 4096 {
		return nil, core.ErrInvalidArgument
	}
	dir, err := unix.Openat2(unix.AT_FDCWD, root, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	defer unix.Close(dir)
	var info unix.Stat_t
	if err := unix.Fstat(dir, &info); err != nil {
		return nil, err
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		return nil, core.ErrInvalidArgument
	}
	fd, err := unix.Openat(dir, ".", unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	writable := os.NewFile(uintptr(fd), "native archive output")
	defer func() {
		if writable != nil {
			err = errors.Join(err, writable.Close())
		}
	}()
	if err := produce(fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var before unix.Stat_t
	if err := unix.Fstat(fd, &before); err != nil {
		return nil, err
	}
	if before.Mode&unix.S_IFMT != unix.S_IFREG || before.Nlink != 0 || before.Size <= 0 || before.Size > limit {
		return nil, core.ErrInvalidArgument
	}
	readFD, err := unix.Open(fmt.Sprintf("/proc/self/fd/%d", fd), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(readFD), "native archive input")
	defer func() {
		if result == nil {
			err = errors.Join(err, file.Close())
		}
	}()
	var after unix.Stat_t
	if err := unix.Fstat(readFD, &after); err != nil {
		return nil, err
	}
	if before.Dev != after.Dev || before.Ino != after.Ino || before.Size != after.Size || after.Nlink != 0 {
		return nil, core.ErrIncompatibleState
	}
	err = writable.Close()
	writable = nil
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, &archiveContextReader{ctx, io.NewSectionReader(file, 0, after.Size)})
	if err != nil {
		return nil, err
	}
	if n != after.Size {
		return nil, core.ErrIncompatibleState
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &NativeArchive{file: file, size: after.Size, digest: hex.EncodeToString(hash.Sum(nil))}, nil
}
