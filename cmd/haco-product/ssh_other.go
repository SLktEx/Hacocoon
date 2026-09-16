//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runSSH([]string) int {
	fmt.Fprintln(os.Stderr, cliMessage("ssh.linux_required"))
	return 1
}
func runOpen(args []string) int { return runSSH(args) }
