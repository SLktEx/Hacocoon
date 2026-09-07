//go:build linux

package recipes

import "syscall"

func makeFIFO(path string) error { return syscall.Mkfifo(path, 0600) }
