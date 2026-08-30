//go:build linux

package ebs

import (
	"errors"
	"os"
	"syscall"
)

func tryExclusiveFileLock(file *os.File) (bool, error) {
	if file == nil {
		return false, syscall.EBADF
	}
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	return false, err
}

func unlockExclusiveFile(file *os.File) error {
	if file == nil {
		return syscall.EBADF
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
