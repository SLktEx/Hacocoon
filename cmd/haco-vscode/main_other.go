//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() { fmt.Fprintln(os.Stderr, "Use the installed WSL/Linux Hacocoon entry."); os.Exit(1) }
