package localqcow2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

const (
	nbdIdentityVersion            = 1
	nbdVerifyAttempts             = 20
	nbdVerifyDelay                = 25 * time.Millisecond
	defaultNBDAllocatorLockPath   = "/run/lock/hacocoon-nbd.lock"
)

var nbdNamePattern = regexp.MustCompile(`^nbd[0-9]+$`)

type nbdIdentity struct {
	Version     int    `json:"version"`
	Device      string `json:"device"`
	BackingPath string `json:"backing_path"`
	BackingDev  uint64 `json:"backing_dev"`
	BackingIno  uint64 `json:"backing_ino"`
}

type backingIdentity struct {
	dev uint64
	ino uint64
}

type nbdObservation int

const (
	nbdFree nbdObservation = iota
	nbdMatches
	nbdOther
	nbdUncertain
)

type nbdInspection struct {
	observation nbdObservation
	pid         int
	reason      string
}

func (s *Store) devRootPath() string {
	if s.devRoot != "" {
		return s.devRoot
	}
	return "/dev"
}

func (s *Store) sysBlockRootPath() string {
	if s.sysBlockRoot != "" {
		return s.sysBlockRoot
	}
	return "/sys/block"
}

func (s *Store) procRootPath() string {
	if s.procRoot != "" {
		return s.procRoot
	}
	return "/proc"
}

func (s *Store) nbdAllocatorLockPath() string {
	if s.nbdLockPath != "" {
		return s.nbdLockPath
	}
	return defaultNBDAllocatorLockPath
}

func nbdStatePath(backing string) string { return backing + ".nbd.json" }

func (s *Store) withNBDAllocatorLock(_ string, fn func() error) error {
	lockPath := s.nbdAllocatorLockPath()
	if !filepath.IsAbs(lockPath) || filepath.Clean(lockPath) != lockPath {
		return errors.Join(fmt.Errorf("global NBD allocator lock path %q is not canonical and absolute", lockPath), core.ErrIncompatibleState)
	}
	fd, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open global NBD allocator lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
	if file == nil {
		_ = syscall.Close(fd)
		return fmt.Errorf("open global NBD allocator lock: %w", core.ErrStorageUnavailable)
	}
	defer file.Close()
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("inspect global NBD allocator lock: %w", err)
	}
	if stat.Uid != uint32(os.Geteuid()) || stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Nlink != 1 || stat.Mode&0o777 != 0o600 {
		return errors.Join(fmt.Errorf("global NBD allocator lock is not trusted"), core.ErrIncompatibleState)
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock global NBD allocator: %w", err)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return fn()
}

func currentBackingIdentity(path string) (backingIdentity, error) {
	if _, err := block.ValidateBackingPath(path, false); err != nil {
		return backingIdentity{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return backingIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return backingIdentity{}, errors.Join(fmt.Errorf("inspect backing inode %q", path), core.ErrIncompatibleState)
	}
	return backingIdentity{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, nil
}

func (s *Store) inspectNBD(device, backing string) nbdInspection {
	name := filepath.Base(device)
	if !nbdNamePattern.MatchString(name) || filepath.Clean(filepath.Dir(device)) != filepath.Clean(s.devRootPath()) {
		return nbdInspection{observation: nbdUncertain, reason: "invalid NBD device path"}
	}
	pidPath := filepath.Join(s.sysBlockRootPath(), name, "pid")
	rawPID, err := os.ReadFile(pidPath)
	if errors.Is(err, os.ErrNotExist) {
		return nbdInspection{observation: nbdFree}
	}
	if err != nil {
		return nbdInspection{observation: nbdUncertain, reason: "cannot read NBD pid"}
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil || pid <= 0 {
		return nbdInspection{observation: nbdUncertain, reason: "invalid NBD pid"}
	}
	cmdline, err := os.ReadFile(filepath.Join(s.procRootPath(), strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nbdInspection{observation: nbdUncertain, pid: pid, reason: "cannot inspect qemu-nbd command line"}
	}
	args := splitProcCmdline(cmdline)
	if len(args) == 0 || filepath.Base(args[0]) != "qemu-nbd" {
		return nbdInspection{observation: nbdUncertain, pid: pid, reason: "active NBD owner is not provably qemu-nbd"}
	}
	connectArg := "--connect=" + device
	if !containsExact(args[1:], connectArg) {
		return nbdInspection{observation: nbdUncertain, pid: pid, reason: "qemu-nbd command line does not bind the expected device"}
	}
	if backing == "" {
		return nbdInspection{observation: nbdOther, pid: pid}
	}
	if !containsExact(args[1:], backing) {
		return nbdInspection{observation: nbdOther, pid: pid}
	}
	expected, err := currentBackingIdentity(backing)
	if err != nil {
		return nbdInspection{observation: nbdUncertain, pid: pid, reason: "cannot identify expected backing image"}
	}
	fdDir := filepath.Join(s.procRootPath(), strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return nbdInspection{observation: nbdUncertain, pid: pid, reason: "cannot inspect qemu-nbd open files"}
	}
	for _, entry := range fds {
		info, err := os.Stat(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if ok && uint64(stat.Dev) == expected.dev && uint64(stat.Ino) == expected.ino {
			return nbdInspection{observation: nbdMatches, pid: pid}
		}
	}
	return nbdInspection{observation: nbdUncertain, pid: pid, reason: "qemu-nbd does not hold the current backing inode"}
}

func splitProcCmdline(raw []byte) []string {
	parts := bytes.Split(raw, []byte{0})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			out = append(out, string(part))
		}
	}
	return out
}

func containsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (s *Store) readNBDIdentity(backing string) (nbdIdentity, bool, error) {
	path := nbdStatePath(backing)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nbdIdentity{}, false, nil
	}
	if err != nil {
		return nbdIdentity{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("NBD identity state %q is not trusted", path), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("NBD identity state %q has unexpected ownership", path), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nbdIdentity{}, false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var state nbdIdentity
	if err := decoder.Decode(&state); err != nil {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("decode NBD identity state: %w", err), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("NBD identity state has trailing JSON"), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	if state.Version != nbdIdentityVersion || state.BackingPath != backing || state.BackingDev == 0 || state.BackingIno == 0 || !s.validNBDDevice(state.Device) {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("NBD identity state does not match the expected backing image"), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	current, err := currentBackingIdentity(backing)
	if err != nil {
		return nbdIdentity{}, false, err
	}
	if state.BackingDev != current.dev || state.BackingIno != current.ino {
		return nbdIdentity{}, false, errors.Join(fmt.Errorf("backing image identity changed while NBD state was persisted"), core.ErrIncompatibleState, core.ErrRecoveryRequired)
	}
	return state, true, nil
}

func (s *Store) writeNBDIdentity(backing, device string) error {
	identity, err := currentBackingIdentity(backing)
	if err != nil {
		return err
	}
	state := nbdIdentity{
		Version:     nbdIdentityVersion,
		Device:      device,
		BackingPath: backing,
		BackingDev:  identity.dev,
		BackingIno:  identity.ino,
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	path := nbdStatePath(backing)
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".nbd-state-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(tmp)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(payload); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	keep = true
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}

func removeNBDIdentity(backing string) error {
	err := os.Remove(nbdStatePath(backing))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) validNBDDevice(device string) bool {
	return filepath.Clean(filepath.Dir(device)) == filepath.Clean(s.devRootPath()) && nbdNamePattern.MatchString(filepath.Base(device))
}

func (s *Store) resolveNBDLocked(handle block.Handle) (string, bool, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return "", false, err
	}
	if handle.Device != "" {
		if !s.validNBDDevice(handle.Device) {
			return "", false, errors.Join(fmt.Errorf("invalid persisted NBD device %q", handle.Device), core.ErrIncompatibleState, core.ErrRecoveryRequired)
		}
		inspection := s.inspectNBD(handle.Device, handle.Path)
		if inspection.observation != nbdMatches {
			return "", false, errors.Join(fmt.Errorf("stale or unprovable NBD handle %s for %s: %s", handle.Device, handle.Path, inspection.reason), core.ErrRecoveryRequired)
		}
		if state, ok, err := s.readNBDIdentity(handle.Path); err != nil {
			return "", false, err
		} else if ok && state.Device != handle.Device {
			return "", false, errors.Join(fmt.Errorf("NBD handle contradicts durable identity state"), core.ErrIncompatibleState, core.ErrRecoveryRequired)
		} else if !ok {
			if err := s.writeNBDIdentity(handle.Path, handle.Device); err != nil {
				return "", false, errors.Join(fmt.Errorf("persist reconstructed NBD identity: %w", err), core.ErrRecoveryRequired)
			}
		}
		return handle.Device, true, nil
	}

	if state, ok, err := s.readNBDIdentity(handle.Path); err != nil {
		return "", false, err
	} else if ok {
		inspection := s.inspectNBD(state.Device, handle.Path)
		switch inspection.observation {
		case nbdMatches:
			return state.Device, true, nil
		case nbdFree:
			if err := removeNBDIdentity(handle.Path); err != nil {
				return "", false, err
			}
		case nbdOther, nbdUncertain:
			return "", false, errors.Join(fmt.Errorf("persisted NBD device %s no longer proves ownership of %s: %s", state.Device, handle.Path, inspection.reason), core.ErrRecoveryRequired)
		}
	}

	matches, uncertain, err := s.scanNBDMappings(handle.Path)
	if err != nil {
		return "", false, err
	}
	if uncertain || len(matches) > 1 {
		return "", false, errors.Join(fmt.Errorf("NBD identity for %s is ambiguous", handle.Path), core.ErrRecoveryRequired)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	if err := s.writeNBDIdentity(handle.Path, matches[0]); err != nil {
		return "", false, errors.Join(fmt.Errorf("persist reconstructed NBD identity: %w", err), core.ErrRecoveryRequired)
	}
	return matches[0], true, nil
}

func (s *Store) scanNBDMappings(backing string) ([]string, bool, error) {
	devices, err := filepath.Glob(filepath.Join(s.devRootPath(), "nbd*"))
	if err != nil {
		return nil, false, err
	}
	var matches []string
	uncertain := false
	for _, device := range devices {
		inspection := s.inspectNBD(device, backing)
		switch inspection.observation {
		case nbdMatches:
			matches = append(matches, device)
		case nbdUncertain:
			uncertain = true
		}
	}
	return matches, uncertain, nil
}

func (s *Store) findFreeNBDLocked() (string, error) {
	devices, err := filepath.Glob(filepath.Join(s.devRootPath(), "nbd*"))
	if err != nil {
		return "", err
	}
	for _, device := range devices {
		name := filepath.Base(device)
		if !nbdNamePattern.MatchString(name) {
			continue
		}
		_, err := os.Stat(filepath.Join(s.sysBlockRootPath(), name, "pid"))
		if errors.Is(err, os.ErrNotExist) {
			return device, nil
		}
		if err != nil {
			return "", errors.Join(fmt.Errorf("cannot prove whether %s is free: %w", device, err), core.ErrRecoveryRequired)
		}
	}
	return "", fmt.Errorf("no free NBD device")
}

func (s *Store) waitForNBDMatch(ctx context.Context, device, backing string) error {
	lastReason := "NBD attachment is not yet observable"
	for i := 0; i < nbdVerifyAttempts; i++ {
		inspection := s.inspectNBD(device, backing)
		switch inspection.observation {
		case nbdMatches:
			return nil
		case nbdOther:
			return errors.Join(fmt.Errorf("cannot verify NBD attachment %s -> %s: device is owned by a different backing", device, backing), core.ErrRecoveryRequired)
		case nbdUncertain:
			if inspection.reason != "" {
				lastReason = inspection.reason
			}
		case nbdFree:
			lastReason = "NBD device is still free"
		}
		if i+1 < nbdVerifyAttempts {
			timer := time.NewTimer(nbdVerifyDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(ctx.Err(), core.ErrRecoveryRequired)
			case <-timer.C:
			}
		}
	}
	return errors.Join(fmt.Errorf("NBD attachment %s -> %s was not provable after bounded observation: %s", device, backing, lastReason), core.ErrRecoveryRequired)
}

func (s *Store) waitForNBDFree(ctx context.Context, device string) error {
	name := filepath.Base(device)
	pidPath := filepath.Join(s.sysBlockRootPath(), name, "pid")
	for i := 0; i < nbdVerifyAttempts; i++ {
		_, err := os.Stat(pidPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return errors.Join(fmt.Errorf("cannot verify NBD detach for %s: %w", device, err), core.ErrRecoveryRequired)
		}
		if i+1 < nbdVerifyAttempts {
			timer := time.NewTimer(nbdVerifyDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(ctx.Err(), core.ErrRecoveryRequired)
			case <-timer.C:
			}
		}
	}
	return errors.Join(fmt.Errorf("NBD device %s remained attached after disconnect", device), core.ErrRecoveryRequired)
}
