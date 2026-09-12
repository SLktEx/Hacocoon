//go:build linux

package workflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"golang.org/x/sys/unix"
)

const ReferenceFile = ".haco-workspace.json"
const referenceLock = ".haco-workspace.lock"

// PathReference is a local navigation reference, never a controller authority.
// A preparation receipt is saved before the first remote mutation.
type PathReference struct {
	Resource core.PersistentResourceRef `json:"resource,omitempty"`
	Version  int                        `json:"version"`
	Reference
	State             string        `json:"state"`
	Repositories      []string      `json:"repositories,omitempty"`
	Base              core.BaseName `json:"base,omitempty"`
	OCI               string        `json:"oci"`
	TemporarySnapshot string        `json:"temporary_snapshot,omitempty"`
}

func NewName() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return "work-" + hex.EncodeToString(b[:])
}

type ReferenceHandle struct{ directory, lock int }

// LockReference pins the selected directory and serializes CLI operations on
// that path. Protected owner-only regular files cannot follow a guest symlink
// or alias another file through a hard link.
func LockReference(ctx context.Context, path string) (*ReferenceHandle, error) {
	dir, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*ReferenceHandle, error) { unix.Close(dir); return nil, e }
	var st unix.Stat_t
	if unix.Fstat(dir, &st) != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&0022 != 0 {
		return fail(fmt.Errorf("Workspace reference directory must be owned by this user and not group/world writable: %w", core.ErrInvalidArgument))
	}
	fd, err := openReferenceFile(dir, referenceLock, unix.O_RDWR|unix.O_CREAT)
	if err != nil {
		return fail(err)
	}
	for {
		if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			unix.Close(fd)
			return fail(err)
		}
		select {
		case <-ctx.Done():
			unix.Close(fd)
			return fail(ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
	// Refuse a lock unlinked/replaced while waiting.
	var current unix.Stat_t
	if unix.Fstat(fd, &st) != nil || unix.Fstatat(dir, referenceLock, &current, unix.AT_SYMLINK_NOFOLLOW) != nil || st.Ino != current.Ino || st.Dev != current.Dev || current.Nlink != 1 {
		unix.Close(fd)
		return fail(core.ErrCapabilityStale)
	}
	return &ReferenceHandle{directory: dir, lock: fd}, nil
}
func openReferenceFile(dir int, name string, flags int) (int, error) {
	fd, err := unix.Openat(dir, name, flags|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return -1, err
	}
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0 {
		unix.Close(fd)
		return -1, fmt.Errorf("unsafe Workspace reference file: %w", core.ErrInvalidArgument)
	}
	return fd, nil
}
func (h *ReferenceHandle) Close() { unix.Close(h.lock); unix.Close(h.directory) }
func (h *ReferenceHandle) Load() (PathReference, error) {
	var ref PathReference
	fd, err := openReferenceFile(h.directory, ReferenceFile, unix.O_RDONLY)
	if err != nil {
		return ref, err
	}
	file := os.NewFile(uintptr(fd), ReferenceFile)
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil {
		return ref, err
	}
	if len(raw) > 8192 {
		return ref, core.ErrInvalidArgument
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&ref) != nil || d.Decode(new(any)) != io.EOF {
		return ref, core.ErrInvalidArgument
	}
	return ref, validateReference(ref)
}
func validateReference(ref PathReference) error {
	if ref.Version != 1 || !gitrepo.ValidID(ref.Name) || (ref.OCI != "none" && ref.OCI != "auto" && !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: ref.OCI, Owner: "0123456789abcdef0123456789abcdef"})) {
		return core.ErrInvalidArgument
	}
	switch ref.State {
	case "preparing":
		if len(ref.Repositories) == 0 {
			return core.ErrInvalidArgument
		}
	case "forking", "recovery-required":
	case "ready":
		if ref.Workspace == "" {
			return core.ErrInvalidArgument
		}
	default:
		return core.ErrInvalidArgument
	}
	return nil
}
func (h *ReferenceHandle) Save(ref PathReference) error {
	if err := validateReference(ref); err != nil {
		return err
	}
	// Never replace an unsafe existing entry, even under our lock.
	fd, err := openReferenceFile(h.directory, ReferenceFile, unix.O_RDONLY)
	if err == nil {
		unix.Close(fd)
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}
	raw, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	name := "." + NewName() + ".tmp"
	fd, err = openReferenceFile(h.directory, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(h.directory, name, 0)
	file := os.NewFile(uintptr(fd), name)
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = unix.Renameat(h.directory, name, h.directory, ReferenceFile); err != nil {
		return err
	}
	return unix.Fsync(h.directory)
}
