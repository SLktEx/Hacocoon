//go:build linux

package main

import (
	"errors"
	"os"
	"syscall"
)

func ownedNotifyFile(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm()&0022 == 0
	}
	return info.Mode().IsRegular() && metadata.Nlink == 1 && info.Mode().Perm()&0077 == 0
}
func openNotifyFile(root *os.Root, name string, flags int, mode os.FileMode) (*os.File, error) {
	file, err := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !ownedNotifyFile(info, false) {
		file.Close()
		return nil, errUnsafeNotifyState
	}
	return file, nil
}
func lockNotifyState(root *os.Root, name string) (func(), error) {
	file, err := openNotifyFile(root, name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("another notification client is using this state")
		}
		return nil, err
	}
	// Keep the inode: removing the lock pathname could allow two independent locks.
	return func() { file.Close() }, nil
}
