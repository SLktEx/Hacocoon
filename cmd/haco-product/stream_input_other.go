//go:build !linux

package main

import (
	"errors"
	"os"
)

func streamInput() (*os.File, error) {
	return nil, errors.New("stream adapter requires the Linux controller client entry")
}
