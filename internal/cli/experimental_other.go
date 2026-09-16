//go:build !linux

package cli

import (
	"fmt"
	"os"
)

func runExperimental([]string) int {
	fmt.Fprintln(os.Stderr, "haco experimental is available in the trusted Linux/WSL Host")
	return 1
}
