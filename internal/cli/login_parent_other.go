//go:build !linux

package cli

func loginBootstrapParent() (bool, error) { return false, nil }
