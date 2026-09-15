//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runConfiguration([]string) int {
	_, _ = fmt.Fprintln(os.Stderr, cliMessage("config.host_required"))
	return 1
}
