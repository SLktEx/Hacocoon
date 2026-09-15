//go:build linux

package main

import (
	"os"
	"syscall"
)

func openCacheInput(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
