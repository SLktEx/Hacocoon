//go:build !linux

package vscodecli

import (
	"fmt"
	"os"
)

func Main() { fmt.Fprintln(os.Stderr, "Use the installed WSL/Linux Hacocoon entry."); os.Exit(1) }
