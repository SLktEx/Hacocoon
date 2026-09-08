//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runConfiguration([]string) int {
	fmt.Fprintln(os.Stderr, "haco config is available in the trusted Linux/WSL Host")
	return 1
}
