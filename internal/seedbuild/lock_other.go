//go:build !linux

package seedbuild

func lockFile(string) (func(), error) { return func() {}, nil }
