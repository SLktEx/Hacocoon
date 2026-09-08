//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runSSH([]string) int {
	fmt.Fprintln(os.Stderr, "haco: SSH setup requires the installed Linux/WSL client")
	return 1
}
func runOpen(args []string) int { return runSSH(args) }
