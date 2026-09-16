//go:build linux && (amd64 || arm64)

package incus

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/sys/unix"
)

// pinnedReclaimTarget holds the actual filesystem, loop and backing file open.
// It never resizes, mounts, detaches or removes objects. Incus still owns
// the storage lifecycle. Only a controller-selected Incus pool may call this.
type pinnedReclaimTarget struct {
	backing, mount, loop *os.File
	source, mountpoint   string
}

func (p *pinnedReclaimTarget) Close() error {
	var errs []error
	for _, f := range []*os.File{p.loop, p.mount, p.backing} {
		if f != nil {
			errs = append(errs, f.Close())
		}
	}
	return errors.Join(errs...)
}

// No symlink component is followed, including an Incus data-directory ancestor.
// Do not emulate openat2 on kernels lacking the required resolution guarantees.
func openReclaimPath(path string, directory bool) (*os.File, error) {
	flags := uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK)
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, &unix.OpenHow{Flags: flags, Resolve: unix.RESOLVE_NO_SYMLINKS})
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func (p *pinnedReclaimTarget) Allocation() (ReclaimAllocation, error) {
	var st unix.Stat_t
	if err := unix.Fstat(int(p.backing.Fd()), &st); err != nil {
		return ReclaimAllocation{}, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size < 0 || st.Blocks < 0 || uint64(st.Blocks) > ^uint64(0)/512 {
		return ReclaimAllocation{}, core.ErrIncompatibleState
	}
	return ReclaimAllocation{uint64(st.Size), uint64(st.Blocks) * 512}, nil
}

func pinReclaimTarget(pool, source string) (_ *pinnedReclaimTarget, err error) {
	if !diagnosticPoolName.MatchString(pool) || !filepath.IsAbs(source) || filepath.Clean(source) != source || !strings.HasSuffix(source, "/disks/"+pool+".img") {
		return nil, core.ErrInvalidArgument
	}
	for _, c := range source {
		if c < 32 || c == 127 {
			return nil, core.ErrInvalidArgument
		}
	}
	p := &pinnedReclaimTarget{source: source, mountpoint: filepath.Join(filepath.Dir(filepath.Dir(source)), "storage-pools", pool)}
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()
	p.backing, err = openReclaimPath(source, false)
	if err != nil {
		return nil, err
	}
	if _, err = p.Allocation(); err != nil {
		return nil, err
	}
	p.mount, err = openReclaimPath(p.mountpoint, true)
	if err != nil {
		return nil, err
	}
	var fs unix.Statfs_t
	if err = unix.Fstatfs(int(p.mount.Fd()), &fs); err != nil {
		return nil, err
	}
	if fs.Type != unix.BTRFS_SUPER_MAGIC {
		return nil, fmt.Errorf("pool is not mounted as Btrfs in this process: %w", core.ErrUnsupported)
	}
	loopPath, err := btrfsSingleDevice(p.mount)
	if err != nil {
		return nil, err
	}
	p.loop, err = openReclaimPath(loopPath, false)
	if err != nil {
		return nil, err
	}
	if err = p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *pinnedReclaimTarget) Validate() error {
	var back, loop unix.Stat_t
	if err := unix.Fstat(int(p.backing.Fd()), &back); err != nil {
		return err
	}
	if err := unix.Fstat(int(p.loop.Fd()), &loop); err != nil {
		return err
	}
	if back.Mode&unix.S_IFMT != unix.S_IFREG || back.Nlink != 1 || loop.Mode&unix.S_IFMT != unix.S_IFBLK {
		return core.ErrIncompatibleState
	}
	association, err := unix.IoctlLoopGetStatus64(int(p.loop.Fd()))
	if err != nil {
		return err
	}
	if association.Device != uint64(back.Dev) || association.Inode != back.Ino || association.Offset != 0 || association.Sizelimit != 0 {
		return core.ErrCapabilityStale
	}
	// Re-open names without following links to detect replacement during review.
	for _, pair := range []struct {
		path string
		held *os.File
		dir  bool
	}{{p.source, p.backing, false}, {p.mountpoint, p.mount, true}} {
		fresh, err := openReclaimPath(pair.path, pair.dir)
		if err != nil {
			return err
		}
		var a, b unix.Stat_t
		ae := unix.Fstat(int(fresh.Fd()), &a)
		be := unix.Fstat(int(pair.held.Fd()), &b)
		ce := fresh.Close()
		if err := errors.Join(ae, be, ce); err != nil {
			return err
		}
		if a.Dev != b.Dev || a.Ino != b.Ino {
			return core.ErrCapabilityStale
		}
	}
	current, err := btrfsSingleDevice(p.mount)
	if err != nil {
		return err
	}
	fresh, err := openReclaimPath(current, false)
	if err != nil {
		return err
	}
	var st unix.Stat_t
	se := unix.Fstat(int(fresh.Fd()), &st)
	ce := fresh.Close()
	if err := errors.Join(se, ce); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFBLK || st.Rdev != loop.Rdev {
		return core.ErrCapabilityStale
	}
	return nil
}

// Linux UAPI: btrfs_ioctl_fs_info_args is 1024 bytes; dev_info_args is 4096.
// These are read-only ioctls, not Btrfs volume-management operations.
// https://github.com/torvalds/linux/blob/v6.6/include/uapi/linux/btrfs.h
func btrfsSingleDevice(mount *os.File) (string, error) {
	var fs [1024]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, mount.Fd(), 0x8400941f, uintptr(unsafe.Pointer(&fs[0])))
	if errno != 0 {
		return "", fmt.Errorf("read Btrfs filesystem identity: %w", errno)
	}
	if binary.NativeEndian.Uint64(fs[8:16]) != 1 || binary.NativeEndian.Uint64(fs[:8]) == 0 {
		return "", core.ErrUnsupported
	}
	var device [4096]byte
	copy(device[:8], fs[:8])
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, mount.Fd(), 0xc0000000|4096<<16|0x94<<8|30, uintptr(unsafe.Pointer(&device[0])))
	if errno != 0 {
		return "", fmt.Errorf("read Btrfs device identity: %w", errno)
	}
	path := device[3072:]
	end := bytes.IndexByte(path, 0)
	if end < 0 {
		return "", core.ErrIncompatibleState
	}
	name := string(path[:end])
	if !diagnosticLoopDevice.MatchString(name) {
		return "", core.ErrUnsupported
	}
	return name, nil
}

// KernelTrimmedBytes describes discard requests, not recovered physical capacity.
// Before/After are the backing file's independently measured allocation.

func (p *pinnedReclaimTarget) Trim(ctx context.Context) (result PoolTrimObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if p == nil || p.backing == nil || p.mount == nil || p.loop == nil {
		return result, core.ErrInvalidArgument
	}
	if err := p.Validate(); err != nil {
		return result, err
	}
	if err := unix.Syncfs(int(p.mount.Fd())); err != nil {
		return result, fmt.Errorf("sync pool before trim: %w", err)
	}
	result.FilesystemBefore, err = reclaimFilesystemUsage(int(p.mount.Fd()), unix.BTRFS_SUPER_MAGIC)
	if err != nil {
		return result, err
	}
	result.Before, err = p.Allocation()
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// FITRIM uses this pinned filesystem, never a reopened caller-selected path.
	// Linux UAPI struct fstrim_range: start, len, minlen, each uint64.
	trimRange := [3]uint64{0, ^uint64(0), 0}
	result.Attempted = true
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, p.mount.Fd(), 0xc0185879, uintptr(unsafe.Pointer(&trimRange[0])))
	if errno == 0 {
		result.KernelReportKnown = true
		result.KernelTrimmedBytes = trimRange[1]
	} else {
		err = fmt.Errorf("trim pool: %w", errno)
	}
	var measureErr error
	result.After, measureErr = p.Allocation()
	var filesystemErr error
	result.FilesystemAfter, filesystemErr = reclaimFilesystemUsage(int(p.mount.Fd()), unix.BTRFS_SUPER_MAGIC)
	identityErr := p.Validate()
	if measureErr == nil && result.Before.LogicalBytes != result.After.LogicalBytes {
		identityErr = errors.Join(identityErr, core.ErrCapabilityStale)
	}
	// The kernel operation can finish after cancellation. Preserve observations
	// and return failure; never claim that a timeout prevented an attempted trim.
	if filesystemErr == nil && result.FilesystemBefore.CapacityBytes != result.FilesystemAfter.CapacityBytes {
		filesystemErr = core.ErrCapabilityStale
	}
	return result, errors.Join(err, measureErr, filesystemErr, identityErr, ctx.Err())
}

// OuterTrimObservation reports filesystem discard, not Windows VHD allocation.
// Filesystem capacity/free space are not the backing VHD's physical size.

// TrimBackingFilesystem forwards discard to the filesystem containing the pinned
// Incus image. The caller must authorize the managed WSL distribution as well as
// the pool: a pool selection alone does not authorize Host-wide filesystem trim.
// The file handle identifies this filesystem without a caller-supplied mountpoint.
// ext4 FITRIM operates on file_inode(file)->i_sb, including a regular-file fd:
// https://github.com/torvalds/linux/blob/v6.6/fs/ext4/ioctl.c
// Other outer filesystems are unsupported, not silently treated as equivalent.
func (p *pinnedReclaimTarget) TrimBackingFilesystem(ctx context.Context) (result OuterTrimObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if p == nil || p.backing == nil || p.mount == nil || p.loop == nil {
		return result, core.ErrInvalidArgument
	}
	if err := p.Validate(); err != nil {
		return result, err
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(p.backing.Fd()), &fs); err != nil {
		return result, err
	}
	if fs.Type != unix.EXT4_SUPER_MAGIC {
		return result, core.ErrUnsupported
	}
	// Flush the hole punching from the inner pool before outer discard.
	if err := unix.Syncfs(int(p.backing.Fd())); err != nil {
		return result, fmt.Errorf("sync outer filesystem before trim: %w", err)
	}
	result.FilesystemBefore, err = reclaimFilesystemUsage(int(p.backing.Fd()), unix.EXT4_SUPER_MAGIC)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	trimRange := [3]uint64{0, ^uint64(0), 0}
	result.Attempted = true
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, p.backing.Fd(), 0xc0185879, uintptr(unsafe.Pointer(&trimRange[0])))
	var trimErr error
	if errno == 0 {
		result.KernelReportKnown = true
		result.KernelTrimmedBytes = trimRange[1]
	} else {
		trimErr = fmt.Errorf("trim outer filesystem: %w", errno)
	}
	// Cancellation cannot undo a kernel operation already attempted. Keep its
	// observation even when the request was canceled during the syscall.
	var measureErr error
	result.FilesystemAfter, measureErr = reclaimFilesystemUsage(int(p.backing.Fd()), unix.EXT4_SUPER_MAGIC)
	if measureErr == nil && result.FilesystemBefore.CapacityBytes != result.FilesystemAfter.CapacityBytes {
		measureErr = core.ErrCapabilityStale
	}
	return result, errors.Join(trimErr, measureErr, p.Validate(), ctx.Err())
}

// Read only through an existing pinned descriptor. This creates no mount or
// storage target and makes filesystem use distinct from physical allocation.
func reclaimFilesystemUsage(fd int, filesystem int64) (*ReclaimFilesystemUsage, error) {
	var fs unix.Statfs_t
	if err := unix.Fstatfs(fd, &fs); err != nil {
		return nil, err
	}
	return reclaimFilesystemCounters(fs, filesystem)
}
func reclaimFilesystemCounters(fs unix.Statfs_t, filesystem int64) (*ReclaimFilesystemUsage, error) {
	if fs.Type != filesystem || fs.Bsize <= 0 || fs.Blocks == 0 || fs.Bfree > fs.Blocks || uint64(fs.Bsize) > ^uint64(0)/fs.Blocks {
		return nil, core.ErrIncompatibleState
	}
	return &ReclaimFilesystemUsage{CapacityBytes: fs.Blocks * uint64(fs.Bsize), UsedBytes: (fs.Blocks - fs.Bfree) * uint64(fs.Bsize)}, nil
}
