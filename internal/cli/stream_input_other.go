//go:build !linux

package cli

import (
	"errors"
	"os"
)

func streamInput() (*os.File, error) {
	return nil, errors.New("stream adapter requires the Linux controller client entry")
}
