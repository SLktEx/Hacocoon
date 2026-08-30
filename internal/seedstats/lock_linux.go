//go:build linux

package seedstats

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func lockStateFile(statePath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return nil, fmt.Errorf("create seed usage state directory: %w", err)
	}
	file, err := os.OpenFile(statePath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open seed usage state lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock seed usage state: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
