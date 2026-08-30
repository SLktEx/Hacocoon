//go:build linux

package ec2

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type workspaceRestoreLock struct {
	file *os.File
}

func acquireWorkspaceRestoreLock(path string) (*workspaceRestoreLock, error) {
	file, err := openOrCreateOwnedLockFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workspace restore lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("workspace restore is already active: %w", core.ErrWorkspaceBusy)
		}
		return nil, fmt.Errorf("lock workspace restore: %w", err)
	}
	return &workspaceRestoreLock{file: file}, nil
}

func (l *workspaceRestoreLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}

func openOrCreateOwnedLockFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	created := err == nil
	if errors.Is(err, syscall.EEXIST) {
		fd, err = syscall.Open(path, syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	}
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if created {
		if err := file.Chmod(0o600); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	if err := verifyOwnedRegularFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func openOwnedRegularFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if err := verifyOwnedRegularFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func createExclusiveOwnedFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := verifyOwnedRegularFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func verifyOwnedRegularFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("workspace restore control file mode/owner/link count is unsafe: %w", core.ErrIncompatibleState)
	}
	return nil
}

func identifyWorkspaceDirectory(path string) (workspaceFileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return workspaceFileIdentity{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return workspaceFileIdentity{}, fmt.Errorf("workspace restore path %s is not a directory: %w", path, core.ErrIncompatibleState)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return workspaceFileIdentity{}, fmt.Errorf("workspace restore identity unavailable: %w", core.ErrUnsupported)
	}
	return workspaceFileIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil
}

func identifyOwnedDirectory(path string, mode os.FileMode) (workspaceFileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return workspaceFileIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm() != mode {
		return workspaceFileIdentity{}, fmt.Errorf("workspace restore transaction directory is unsafe: %w", core.ErrIncompatibleState)
	}
	return workspaceFileIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil
}

func renameWorkspaceNoReplace(oldPath, newPath string) error {
	oldPtr, err := syscall.BytePtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPtr, err := syscall.BytePtrFromString(newPath)
	if err != nil {
		return err
	}
	atFDCWD := -100
	_, _, errno := syscall.Syscall6(
		renameat2Syscall,
		uintptr(atFDCWD), uintptr(unsafe.Pointer(oldPtr)),
		uintptr(atFDCWD), uintptr(unsafe.Pointer(newPtr)),
		uintptr(1), 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
