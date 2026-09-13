//go:build linux

package main

import (
	"golang.org/x/sys/unix"
	"os"
)

// Own a pollable descriptor so remote EOF/cancellation can interrupt a pending
// pipe read. os.Stdin itself can be a blocking kindNewFile descriptor.
func streamInput() (*os.File, error) {
	fd, err := unix.FcntlInt(os.Stdin.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), "stream-stdin"), nil
}
