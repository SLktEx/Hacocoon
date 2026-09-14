//go:build linux

package experimental

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

type Store struct{ Path string }
type Snapshot struct {
	Revision string
	Object   map[string]any
	Present  bool
}

func DefaultStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: filepath.Join(dir, "hacocoon", "config.yaml")}, nil
}

func safeFile(f *os.File, directory bool) bool {
	s, err := f.Stat()
	if err != nil {
		return false
	}
	t, ok := s.Sys().(*syscall.Stat_t)
	return ok && t.Uid == uint32(os.Geteuid()) && s.Mode().Perm()&0022 == 0 &&
		((directory && s.IsDir()) || (!directory && s.Mode().IsRegular() && t.Nlink == 1))
}

func (s Store) open(create bool) (*os.Root, error) {
	if !filepath.IsAbs(s.Path) {
		return nil, fmt.Errorf("configuration path must be absolute")
	}
	dir := filepath.Dir(s.Path)
	if create {
		if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
			return nil, err
		}
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	f, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if !safeFile(f, true) {
		return nil, fmt.Errorf("unsafe configuration directory")
	}
	// Pin the validated directory inode, including across renames.
	return os.OpenRoot(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
}

func read(root *os.Root, name string) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if !safeFile(f, false) {
		return nil, fmt.Errorf("unsafe configuration file")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil || len(b) > MaxBytes {
		return nil, fmt.Errorf("cannot read bounded configuration")
	}
	return b, nil
}

func snapshot(data []byte) (Snapshot, error) {
	h := sha256.New()
	if data != nil {
		h.Write([]byte{1})
	}
	h.Write(data)
	s := Snapshot{Revision: fmt.Sprintf("%x", h.Sum(nil)), Object: map[string]any{}}
	if data == nil {
		return s, nil
	}
	m, err := DecodeObject(data)
	if err != nil {
		return Snapshot{}, err
	}
	if raw, exists := m["experimental"]; exists {
		x, ok := raw.(map[string]any)
		if !ok {
			return Snapshot{}, fmt.Errorf("experimental must be an object")
		}
		if raw, exists := x["vscode"]; exists {
			v, ok := raw.(map[string]any)
			if !ok {
				return Snapshot{}, fmt.Errorf("experimental.vscode must be an object")
			}
			if _, err := VSCodeFromObject(v); err != nil {
				return Snapshot{}, err
			}
			s.Object, s.Present = v, true
		}
	}
	return s, nil
}

func (s Store) Read(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	r, err := s.open(false)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot(nil)
	}
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = r.Close() }()
	b, err := read(r, filepath.Base(s.Path))
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(b)
}

// Replace changes only the subtree, under a shared lock and revision comparison.
// The YAML remains the only configuration source. Comments are not retained.
func (s Store) Replace(ctx context.Context, revision string, object map[string]any) error {
	if _, err := VSCodeFromObject(object); err != nil {
		return err
	}
	r, err := s.open(true)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	lock, err := r.OpenFile(".config.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	if !safeFile(lock, false) {
		return fmt.Errorf("unsafe configuration lock")
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	before, err := read(r, filepath.Base(s.Path))
	if err != nil {
		return err
	}
	current, err := snapshot(before)
	if err != nil {
		return err
	}
	if revision != current.Revision {
		return fmt.Errorf("configuration changed; read it again before retrying")
	}
	document := map[string]any{}
	if before != nil {
		document, err = DecodeObject(before)
		if err != nil {
			return err
		}
	}
	x, ok := document["experimental"].(map[string]any)
	if !ok {
		x = map[string]any{}
		document["experimental"] = x
	}
	x["vscode"] = object
	b, err := yaml.Marshal(document)
	if err != nil || len(b) > MaxBytes {
		return fmt.Errorf("invalid configuration size")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := fmt.Sprintf(".config-%x", nonce)
	f, err := r.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = r.Remove(temp) }()
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.Rename(temp, filepath.Base(s.Path)); err != nil {
		return err
	}
	d, err := r.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}
