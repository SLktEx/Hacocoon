//go:build linux

package sshclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type files struct {
	root *os.Root
	lock *os.File
}

func openFiles(ctx context.Context, home string, windows bool) (*files, error) {
	dir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe SSH directory")
	}
	if !windows && (!owned(info) || info.Mode().Perm()&0022 != 0) {
		return nil, fmt.Errorf("SSH directory is writable by other users")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	f := &files{root: root}
	if err = root.Mkdir("hacocoon", 0700); err != nil && !errors.Is(err, os.ErrExist) {
		f.close()
		return nil, err
	}
	info, err = root.Lstat("hacocoon")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		f.close()
		return nil, fmt.Errorf("unsafe managed SSH directory")
	}
	if !windows && (!owned(info) || info.Mode().Perm()&0077 != 0) {
		f.close()
		return nil, fmt.Errorf("managed SSH directory must be private")
	}
	lock, err := root.OpenFile("hacocoon/setup.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		f.close()
		return nil, err
	}
	f.lock = lock
	if err = regular(lock); err != nil {
		f.close()
		return nil, err
	}
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			f.close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func regular(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || !ok || stat.Nlink != 1 {
		return fmt.Errorf("SSH file must be a regular file with one link")
	}
	return nil
}
func (f *files) close() {
	if f.lock != nil {
		f.lock.Close()
	}
	if f.root != nil {
		f.root.Close()
	}
}
func (f *files) read(name string) ([]byte, error) {
	file, err := f.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err = regular(file); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1024*1024 {
		return nil, fmt.Errorf("SSH file exceeds size limit")
	}
	return b, nil
}
func (f *files) replace(name string, b []byte) error {
	if _, err := f.read(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := "hacocoon/.write-" + hex.EncodeToString(nonce[:])
	file, err := f.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.root.Remove(temp)
	_, err = file.Write(b)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return f.root.Rename(temp, name)
}

func owned(info os.FileInfo) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Uid == uint32(os.Geteuid())
}
