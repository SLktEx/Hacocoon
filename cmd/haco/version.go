package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Version handling is intentionally performed before main initializes the local
// runtime composition. `haco --version` and `haco version` must work even when
// Incus or Host state is unavailable.
func init() {
	handled, err := handleVersionArgs(os.Args[1:], os.Stdout)
	if !handled {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		os.Exit(2)
	}
	os.Exit(0)
}

func handleVersionArgs(args []string, out io.Writer) (bool, error) {
	if len(args) == 1 && args[0] == "--version" {
		info := buildinfo.Current()
		_, err := fmt.Fprintf(out, "haco %s (checkpoint %s, commit %s)\n", info.Version, info.Checkpoint, buildinfo.ShortCommit(info.Commit))
		return true, err
	}
	if len(args) == 0 || args[0] != "version" {
		return false, nil
	}
	args = args[1:]
	jsonOutput := false
	if len(args) == 1 && args[0] == "--json" {
		jsonOutput = true
		args = args[:0]
	}
	if len(args) != 0 {
		return true, fmt.Errorf("usage: haco version [--json]: %w", core.ErrInvalidArgument)
	}

	info := buildinfo.Current()
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetEscapeHTML(false)
		return true, encoder.Encode(info)
	}
	_, err := fmt.Fprintf(out,
		"Hacocoon\n  checkpoint: %s\n  version: %s\n  commit: %s\n  built: %s\n",
		info.Checkpoint, info.Version, info.Commit, info.BuildDate,
	)
	return true, err
}
