//go:build !linux

package main

func loginBootstrapParent() (bool, error) { return false, nil }
