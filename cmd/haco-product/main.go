package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
)

const loginAlias = "hacocoon-login"

func main() {
	if isLoginAlias(os.Args[0]) {
		if err := runLoginShim(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			os.Exit(1)
		}
		return
	}

	code := run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

func run(args []string) int {
	if len(args) == 0 {
		writeHelp(os.Stdout)
		return 0
	}

	switch args[0] {
	case "--help", "-h", "help":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "haco: usage: haco help")
			return 2
		}
		writeHelp(os.Stdout)
		return 0
	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "haco: usage: haco --version")
			return 2
		}
		writeShortVersion()
		return 0
	case "version":
		return runVersion(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "haco: command %q is not available yet; run 'haco help'\n", args[0])
		return 2
	}
}

func runVersion(args []string) int {
	info := buildinfo.Current()
	if len(args) == 0 {
		fmt.Printf("Hacocoon\n  checkpoint: %s\n  version: %s\n  commit: %s\n  built: %s\n",
			info.Checkpoint, info.Version, info.Commit, info.BuildDate)
		return 0
	}
	if len(args) == 1 && args[0] == "--json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(info); err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "haco: usage: haco version [--json]")
	return 2
}

func writeShortVersion() {
	info := buildinfo.Current()
	fmt.Printf("haco %s (checkpoint %s, commit %s)\n", info.Version, info.Checkpoint, buildinfo.ShortCommit(info.Commit))
}

func writeHelp(out *os.File) {
	fmt.Fprintln(out, "Hacocoon")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  haco <command>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  help       Show this help")
	fmt.Fprintln(out, "  version    Show Hacocoon version information")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "The product CLI is being rebuilt from the basic workflow outward.")
}

func isLoginAlias(argv0 string) bool {
	name := strings.TrimPrefix(filepath.Base(argv0), "-")
	return name == loginAlias
}

func runLoginShim(args []string) error {
	// Explicit shell arguments and non-interactive callers remain on the WSL
	// Physical Host. Only a normal interactive distro entry is redirected into
	// the trusted haco-host.
	if len(args) != 0 || !stdioIsInteractive() {
		return execProcess("/bin/bash", append([]string{"bash"}, args...))
	}

	legacy := "/usr/local/bin/hacoq"
	if configured := strings.TrimSpace(os.Getenv("HACOQ_PATH")); configured != "" {
		legacy = configured
	}
	if err := syscall.Exec(legacy, []string{"hacoq", "host", "shell"}, os.Environ()); err != nil {
		return fmt.Errorf("enter haco-host through temporary compatibility CLI %q: %w", legacy, err)
	}
	return nil
}

func stdioIsInteractive() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil || stdin.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	stdout, err := os.Stdout.Stat()
	return err == nil && stdout.Mode()&os.ModeCharDevice != 0
}

func execProcess(path string, argv []string) error {
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}
