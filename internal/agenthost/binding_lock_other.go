//go:build !linux

package agenthost

func lockBindingState(_ string) (func(), error) {
	return func() {}, nil
}
