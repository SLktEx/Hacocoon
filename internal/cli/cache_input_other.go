//go:build !linux

package cli

import "os"

func openCacheInput(name string) (*os.File, error) { return os.Open(name) }
