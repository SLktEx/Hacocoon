//go:build linux

package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

func settingsFileSafe(file *os.File, directory bool) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0077 == 0 && ((directory && info.IsDir()) || (!directory && info.Mode().IsRegular() && stat.Nlink == 1))
}

func (s Settings) open(create bool) (*os.Root, error) {
	if !filepath.IsAbs(s.Path) {
		return nil, core.ErrInvalidArgument
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
			return nil, err
		}
	}
	dir, err := os.OpenFile(filepath.Dir(s.Path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	if !settingsFileSafe(dir, true) {
		return nil, core.ErrIncompatibleState
	}
	// Pin the already checked directory rather than reopening its mutable path.
	return os.OpenRoot("/proc/self/fd/" + strconv.FormatUint(uint64(dir.Fd()), 10))
}

func readSettings(root *os.Root, name string) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if !settingsFileSafe(f, false) {
		return nil, core.ErrIncompatibleState
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxConfigurationBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxConfigurationBytes {
		return nil, core.ErrInvalidArgument
	}
	return data, nil
}

func settingsSnapshot(data []byte) (SettingsSnapshot, error) {
	result := SettingsSnapshot{Revision: settingsRevision(data), Configuration: Configuration{Areas: []Area{}}}
	if data == nil {
		return result, nil
	}
	configuration, err := DecodeConfiguration(data)
	result.Configuration = configuration
	return result, err
}

func (s Settings) Read(ctx context.Context) (SettingsSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return SettingsSnapshot{}, err
	}
	root, err := s.open(false)
	if errors.Is(err, os.ErrNotExist) {
		return settingsSnapshot(nil)
	}
	if err != nil {
		return SettingsSnapshot{}, err
	}
	defer func() { _ = root.Close() }()
	data, err := readSettings(root, filepath.Base(s.Path))
	if err != nil {
		return SettingsSnapshot{}, err
	}
	return settingsSnapshot(data)
}

func (s Settings) Replace(ctx context.Context, edit SettingsSnapshot) (SettingsSnapshot, error) {
	if len(edit.Revision) != 64 {
		return SettingsSnapshot{}, core.ErrInvalidArgument
	}
	data, err := json.Marshal(edit.Configuration)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if _, err = DecodeConfiguration(data); err != nil {
		return SettingsSnapshot{}, err
	}
	if err = ctx.Err(); err != nil {
		return SettingsSnapshot{}, err
	}
	root, err := s.open(true)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	defer func() { _ = root.Close() }()
	lock, err := root.OpenFile(filepath.Base(s.Path)+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	defer func() { _ = lock.Close() }()
	if !settingsFileSafe(lock, false) {
		return SettingsSnapshot{}, core.ErrIncompatibleState
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return SettingsSnapshot{}, core.ErrStorageBusy
	}
	before, err := readSettings(root, filepath.Base(s.Path))
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if settingsRevision(before) != edit.Revision {
		return SettingsSnapshot{}, core.ErrCapabilityStale
	}
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	name := ".cache-settings-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	defer func() { _ = root.Remove(name) }()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if err = ctx.Err(); err != nil {
		return SettingsSnapshot{}, err
	}
	if err = root.Rename(name, filepath.Base(s.Path)); err != nil {
		return SettingsSnapshot{}, err
	}
	dir, err := root.Open(".")
	if err != nil {
		return SettingsSnapshot{}, errors.Join(core.ErrRecoveryRequired, err)
	}
	err = errors.Join(dir.Sync(), dir.Close())
	if err != nil {
		return SettingsSnapshot{}, errors.Join(core.ErrRecoveryRequired, err)
	}
	return settingsSnapshot(data)
}
