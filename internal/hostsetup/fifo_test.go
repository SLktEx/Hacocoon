//go:build linux

package hostsetup

import "syscall"

func makeFIFO(path string) error { return syscall.Mkfifo(path, 0600) }
