//go:build !linux

package main

import "os"

// Native clients are currently packaged for Linux (including WSL). Other
// platforms must implement the same ownership and locking guarantees first.
func ownedNotifyFile(os.FileInfo, bool) bool { return false }
func openNotifyFile(*os.Root, string, int, os.FileMode) (*os.File, error) {
	return nil, errUnsafeNotifyState
}
func lockNotifyState(*os.Root, string) (func(), error) { return nil, errUnsafeNotifyState }
