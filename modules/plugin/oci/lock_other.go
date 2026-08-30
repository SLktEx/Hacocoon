//go:build !linux

package oci

func lockStateFile(string) (func(), error) {
	return func() {}, nil
}
