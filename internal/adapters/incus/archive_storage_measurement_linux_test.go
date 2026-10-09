//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	environmenttransfer "github.com/SLktEx/Hacocoon/internal/env/transfer"
)

// This matches the envelope allowance enforced by environmenttransfer.Inspect.
// The caller supplies the fixture's existing aggregate component budget.
const archiveMeasurementEnvelopeBudget int64 = 512 << 10

type archiveStorageComponent struct {
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// AllocatedBytes is st_blocks * 512: referenced allocation for this file, not
// exclusive storage, compressed extent size, or an operation's physical writes.
// Keep this explicit projection separate from the private verified manifest.
type archiveStorageSample struct {
	LogicalBytes             uint64                    `json:"logical_bytes"`
	AllocatedBytes           uint64                    `json:"allocated_bytes"`
	SHA256                   string                    `json:"sha256"`
	FilesystemType           int64                     `json:"filesystem_type"`
	FilesystemIdentitySHA256 string                    `json:"filesystem_identity_sha256"`
	FileIdentitySHA256       string                    `json:"file_identity_sha256"`
	Components               []archiveStorageComponent `json:"components"`
	ComponentBytes           uint64                    `json:"component_bytes"`
	EnvelopeOverheadBytes    uint64                    `json:"envelope_overhead_bytes"`
}

// This observer owns only its read-only descriptor. It never extracts, stages,
// copies, repairs or deletes the retained CLI output. The pinned descriptor does
// not make the public pathname immutable; Observe must bracket every import and
// cleanup. A private fixture directory and unchanged ancestors are mandatory.
type retainedArchiveStorage struct {
	mu        sync.Mutex
	file      *os.File
	path      string
	limit     int64
	identity  syscall.Stat_t
	ancestors []syscall.Stat_t
	baseline  archiveStorageSample
}

func openArchiveStorageMeasurement(ctx context.Context, path string, componentLimit int64) (*retainedArchiveStorage, environmenttransfer.Manifest, archiveStorageSample, error) {
	if ctx == nil || componentLimit <= 0 || componentLimit > math.MaxInt64-archiveMeasurementEnvelopeBudget {
		return nil, environmenttransfer.Manifest{}, archiveStorageSample{}, errors.New("invalid archive observation budget or context")
	}
	if err := ctx.Err(); err != nil {
		return nil, environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	file, ancestors, stat, err := openArchiveMeasurementFile(path)
	if err != nil {
		return nil, environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	retained := &retainedArchiveStorage{file: file, path: path, limit: componentLimit, identity: stat, ancestors: ancestors}
	manifest, sample, err := retained.observe(ctx)
	if err != nil {
		_ = retained.Close()
		return nil, environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	retained.baseline = sample
	return retained, manifest, sample, nil
}

// Walk from a freshly opened /, refusing symlinks atomically at every component.
// Nonblocking opens ensure a replaced FIFO cannot hang the observer. Ancestors
// must be owned by root or the observer and not writable by another principal;
// a root-owned sticky ancestor such as /tmp is safe for an owned child. The
// immediate fixture parent must be private and owned by the observer itself.
func openArchiveMeasurementFile(path string) (*os.File, []syscall.Stat_t, syscall.Stat_t, error) {
	fail := func() (*os.File, []syscall.Stat_t, syscall.Stat_t, error) {
		return nil, nil, syscall.Stat_t{}, errors.New("archive path, parent or file identity unavailable or unsafe")
	}
	if len(path) > 4096 || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return fail()
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fail()
	}
	defer func() { _ = syscall.Close(fd) }()
	var ancestors []syscall.Stat_t
	for i := 0; i < len(parts); i++ {
		var stat syscall.Stat_t
		if syscall.Fstat(fd, &stat) != nil || !safeArchiveMeasurementDirectory(stat, i == len(parts)-1) {
			return fail()
		}
		ancestors = append(ancestors, stat)
		if i == len(parts)-1 {
			break
		}
		next, err := syscall.Openat(fd, parts[i], syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return fail()
		}
		closeErr := syscall.Close(fd)
		fd = next
		if closeErr != nil {
			return fail()
		}
	}
	archiveFD, err := syscall.Openat(fd, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fail()
	}
	file := os.NewFile(uintptr(archiveFD), "retained archive observation")
	var stat syscall.Stat_t
	if syscall.Fstat(archiveFD, &stat) != nil || !safeArchiveMeasurementRegular(stat) {
		_ = file.Close()
		return fail()
	}
	return file, ancestors, stat, nil
}

func safeArchiveMeasurementDirectory(stat syscall.Stat_t, immediate bool) bool {
	uid := uint32(os.Geteuid())
	if stat.Mode&syscall.S_IFMT != syscall.S_IFDIR || stat.Uid != 0 && stat.Uid != uid {
		return false
	}
	if immediate {
		return stat.Uid == uid && stat.Mode&0077 == 0
	}
	return stat.Mode&0022 == 0 || stat.Uid == 0 && stat.Mode&syscall.S_ISVTX != 0
}

func safeArchiveMeasurementRegular(stat syscall.Stat_t) bool {
	return stat.Mode&syscall.S_IFMT == syscall.S_IFREG && stat.Nlink == 1 && stat.Uid == uint32(os.Geteuid()) && stat.Mode&0022 == 0 && stat.Size > 0 && stat.Blocks >= 0
}

func sameArchiveMeasurementIdentity(a, b syscall.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid && a.Gid == b.Gid
}

func sameArchiveMeasurementFile(a, b syscall.Stat_t) bool {
	return sameArchiveMeasurementIdentity(a, b) && a.Size == b.Size && a.Nlink == b.Nlink
}

func (r *retainedArchiveStorage) binding() (syscall.Stat_t, error) {
	fail := func() (syscall.Stat_t, error) {
		return syscall.Stat_t{}, errors.New("retained archive identity, ownership or parent changed")
	}
	var pinned syscall.Stat_t
	if syscall.Fstat(int(r.file.Fd()), &pinned) != nil || !safeArchiveMeasurementRegular(pinned) || !sameArchiveMeasurementFile(pinned, r.identity) {
		return fail()
	}
	current, ancestors, stat, err := openArchiveMeasurementFile(r.path)
	if err != nil {
		return fail()
	}
	if current.Close() != nil || !sameArchiveMeasurementFile(stat, pinned) || len(ancestors) != len(r.ancestors) {
		return fail()
	}
	for i := range ancestors {
		if !sameArchiveMeasurementIdentity(ancestors[i], r.ancestors[i]) {
			return fail()
		}
	}
	return pinned, nil
}

// Reads are bounded by the original file size, the bundle budget and 64 KiB per
// read. Cancellation is checked between regular-file reads, without a goroutine
// that could outlive a failed observation. It cannot interrupt a blocked kernel
// filesystem read; it is not an I/O deadline for a stalled underlying device.
type archiveMeasurementReader struct {
	ctx context.Context
	src io.Reader
}

func (r archiveMeasurementReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	return r.src.Read(p)
}

func archiveMeasurementIdentityDigest(domain string, values ...any) string {
	hash := sha256.Sum256([]byte(domain + fmt.Sprint(values...)))
	return hex.EncodeToString(hash[:])
}

func (r *retainedArchiveStorage) observe(ctx context.Context) (environmenttransfer.Manifest, archiveStorageSample, error) {
	fail := func(message string) (environmenttransfer.Manifest, archiveStorageSample, error) {
		return environmenttransfer.Manifest{}, archiveStorageSample{}, errors.New(message)
	}
	if ctx == nil || r.file == nil {
		return fail("archive observation context or descriptor unavailable")
	}
	if err := ctx.Err(); err != nil {
		return environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	before, err := r.binding()
	if err != nil || before.Size > r.limit+archiveMeasurementEnvelopeBudget {
		return fail("retained archive identity or bounded size changed")
	}
	var filesystem syscall.Statfs_t
	if syscall.Fstatfs(int(r.file.Fd()), &filesystem) != nil {
		return fail("archive filesystem observation unavailable")
	}
	digest := sha256.New()
	reader := archiveMeasurementReader{ctx: ctx, src: io.NewSectionReader(r.file, 0, before.Size)}
	manifest, inspectErr := environmenttransfer.Inspect(io.TeeReader(reader, digest), r.limit)
	if err := ctx.Err(); err != nil {
		return environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	if inspectErr != nil {
		return fail("retained archive bundle verification failed")
	}
	after, err := r.binding()
	if err != nil || !sameArchiveMeasurementFile(before, after) || before.Mtim != after.Mtim || before.Ctim != after.Ctim {
		return fail("retained archive changed during observation")
	}
	var filesystemAfter syscall.Statfs_t
	if syscall.Fstatfs(int(r.file.Fd()), &filesystemAfter) != nil || filesystem.Type != filesystemAfter.Type || filesystem.Fsid != filesystemAfter.Fsid {
		return fail("archive filesystem identity changed during observation")
	}
	sample, err := archiveMeasurementSample(after, filesystemAfter, manifest.Components, hex.EncodeToString(digest.Sum(nil)))
	if err != nil {
		return environmenttransfer.Manifest{}, archiveStorageSample{}, err
	}
	if r.baseline.SHA256 != "" && (sample.SHA256 != r.baseline.SHA256 || sample.FilesystemIdentitySHA256 != r.baseline.FilesystemIdentitySHA256 || sample.FileIdentitySHA256 != r.baseline.FileIdentitySHA256) {
		return fail("retained archive content or filesystem identity changed")
	}
	return manifest, sample, nil
}

func archiveMeasurementSample(stat syscall.Stat_t, filesystem syscall.Statfs_t, components []environmenttransfer.Component, digest string) (archiveStorageSample, error) {
	if stat.Size <= 0 || stat.Blocks < 0 || uint64(stat.Blocks) > math.MaxUint64/512 {
		return archiveStorageSample{}, errors.New("invalid archive size or referenced allocation")
	}
	sample := archiveStorageSample{
		LogicalBytes: uint64(stat.Size), AllocatedBytes: uint64(stat.Blocks) * 512, SHA256: digest,
		FilesystemType:           int64(filesystem.Type),
		FilesystemIdentitySHA256: archiveMeasurementIdentityDigest("archive-filesystem-v1:", stat.Dev, filesystem.Type, filesystem.Fsid),
		FileIdentitySHA256:       archiveMeasurementIdentityDigest("archive-file-v1:", stat.Dev, stat.Ino),
	}
	for _, component := range components {
		if component.Bytes <= 0 || uint64(component.Bytes) > sample.LogicalBytes-sample.ComponentBytes {
			return archiveStorageSample{}, errors.New("invalid archive component byte accounting")
		}
		sample.ComponentBytes += uint64(component.Bytes)
		sample.Components = append(sample.Components, archiveStorageComponent{Role: component.Role, Bytes: component.Bytes, SHA256: component.SHA256})
	}
	sample.EnvelopeOverheadBytes = sample.LogicalBytes - sample.ComponentBytes
	if len(sample.Components) < 2 || sample.EnvelopeOverheadBytes == 0 || sample.EnvelopeOverheadBytes > uint64(archiveMeasurementEnvelopeBudget) {
		return archiveStorageSample{}, errors.New("invalid archive envelope byte accounting")
	}
	return sample, nil
}

func (r *retainedArchiveStorage) Observe(ctx context.Context) (archiveStorageSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, sample, err := r.observe(ctx)
	return sample, err
}

func (r *retainedArchiveStorage) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	if err != nil {
		return errors.New("archive observation descriptor close failed")
	}
	return nil
}
