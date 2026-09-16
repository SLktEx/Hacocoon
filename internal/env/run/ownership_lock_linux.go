//go:build linux

package run

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type runOwnershipLock interface {
	Release() error
}

type fileOwnershipLock struct {
	file *os.File
}

func acquireOwnershipLock(dir, environmentID string, nonBlocking bool) (runOwnershipLock, bool, error) {
	if dir == "" || environmentID == "" {
		return nil, false, fmt.Errorf("ephemeral run ownership lock: invalid path or environment")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, fmt.Errorf("create ephemeral run lock directory: %w", err)
	}
	name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(environmentID)))
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open ephemeral run ownership lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("secure ephemeral run ownership lock: %w", err)
	}
	operation := syscall.LOCK_EX
	if nonBlocking {
		operation |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(file.Fd()), operation); err != nil {
		_ = file.Close()
		if nonBlocking && (errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lock ephemeral run ownership: %w", err)
	}
	return &fileOwnershipLock{file: file}, true, nil
}

func (l *fileOwnershipLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}
