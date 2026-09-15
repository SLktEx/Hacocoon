//go:build !linux

package main

import "os"

func openCacheInput(name string) (*os.File, error) { return os.Open(name) }
