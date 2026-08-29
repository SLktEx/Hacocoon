//go:build !linux

package state

func lockEnvironmentState(_ string) (func(), error) {
	return func() {}, nil
}
