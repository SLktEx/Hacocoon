//go:build !linux

package seedstats

func lockStateFile(string) (func(), error) {
	return func() {}, nil
}
