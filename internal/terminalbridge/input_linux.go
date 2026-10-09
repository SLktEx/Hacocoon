//go:build linux

package terminalbridge

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// ownInput gives the bridge a descriptor it can close without closing stdin or
// changing the caller's file-status flags. dup alone is insufficient for pipes
// and terminals: setting O_NONBLOCK on a duplicate also changes the original.
// Regular files and other readers retain caller ownership and their offsets;
// non-pollable reads cannot safely be interrupted and joined here.
func ownInput(input io.Reader) (io.Reader, func(), error) {
	file, ok := input.(*os.File)
	if !ok {
		return input, nil, nil
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, nil, err
	}
	fd := -1
	pollable := false
	terminal := false
	terminalDevice := 0
	var openErr error
	err = raw.Control(func(source uintptr) {
		flags, err := unix.FcntlInt(source, unix.F_GETFL, 0)
		if err != nil {
			openErr = err
			return
		}
		if flags&unix.O_ACCMODE == unix.O_WRONLY || flags&unix.O_PATH != 0 {
			openErr = unix.EBADF
			return
		}
		var before unix.Stat_t
		if openErr = unix.Fstat(int(source), &before); openErr != nil {
			return
		}
		switch before.Mode & unix.S_IFMT {
		case unix.S_IFIFO:
			pollable = true
		case unix.S_IFCHR:
			terminal = term.IsTerminal(int(source))
			pollable = terminal
			if terminal {
				// /dev/ptmx is a clone device: reopening it allocates a new
				// terminal even though fstat reports the same device/inode.
				if _, err := unix.IoctlGetInt(int(source), unix.TIOCGPTN); err == nil {
					openErr = errors.New("local input PTY master cannot be reopened")
					return
				}
				terminalDevice, openErr = unix.IoctlGetInt(int(source), unix.TIOCGDEV)
				if openErr != nil {
					return
				}
			}
		}
		if !pollable {
			return
		}
		fd, openErr = unix.Open(fmt.Sprintf("/proc/self/fd/%d", source), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
		if openErr != nil {
			return
		}
		var after unix.Stat_t
		openErr = unix.Fstat(fd, &after)
		if openErr == nil && (before.Dev != after.Dev || before.Ino != after.Ino || before.Rdev != after.Rdev || before.Mode&unix.S_IFMT != after.Mode&unix.S_IFMT) {
			openErr = errors.New("local input identity changed")
		}
		if openErr == nil && terminal {
			var device int
			device, openErr = unix.IoctlGetInt(fd, unix.TIOCGDEV)
			if openErr == nil && device != terminalDevice {
				openErr = errors.New("local input terminal identity changed")
			}
		}
	})
	if err != nil || openErr != nil {
		if fd >= 0 {
			_ = unix.Close(fd)
		}
		return nil, nil, fmt.Errorf("own local input: %w", errors.Join(err, openErr))
	}
	if fd < 0 {
		// Regular files, sockets and non-terminal devices retain existing
		// ownership. Closing a regular file cannot interrupt a stalled read.
		return input, nil, nil
	}
	owned := os.NewFile(uintptr(fd), "terminal-input")
	if pollable {
		// Refuse a descriptor the Go poller cannot interrupt instead of
		// starting another indefinitely blocked input-copy goroutine.
		if err := owned.SetReadDeadline(time.Time{}); err != nil {
			_ = owned.Close()
			return nil, nil, fmt.Errorf("poll local input: %w", err)
		}
	}
	return owned, func() { _ = owned.Close() }, nil
}
