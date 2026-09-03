package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const hacocoonLoginName = "hacocoon-login"

func isHacocoonLogin(argv0 string) bool {
	name := strings.TrimPrefix(filepath.Base(argv0), "-")
	return name == hacocoonLoginName
}

func runHacocoonLogin(args []string) error {
	// Explicit shell arguments and non-interactive callers stay on the Physical
	// Host. This preserves `wsl -d Hacocoon -- <command>` and tool automation
	// without injecting the interactive haco-host warning into their output.
	if len(args) != 0 || !stdioIsInteractive() {
		return execProcess("/bin/bash", append([]string{"bash"}, args...))
	}

	legacyBinary, err := os.Executable()
	if err != nil {
		return wslLoginRecoveryError(fmt.Errorf("resolve hacoq executable: %w", err))
	}
	if resolved, resolveErr := filepath.EvalSymlinks(legacyBinary); resolveErr == nil {
		legacyBinary = resolved
	}
	// `hacoq host shell` is intercepted before runtime composition and connects
	// to the Physical Host controller as the ordinary hacocoon-group user. It
	// does not require sudo or grant the migration CLI extra Host authority.
	if err := syscall.Exec(legacyBinary, []string{"hacoq", "host", "shell"}, os.Environ()); err != nil {
		return wslLoginRecoveryError(fmt.Errorf("enter haco-host through temporary hacoq login shim: %w", err))
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

func wslLoginRecoveryError(cause error) error {
	distro := strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME"))
	if distro == "" {
		distro = "Hacocoon"
	}
	return fmt.Errorf("%w\nPhysical Host recovery: wsl -d %s -u root", cause, distro)
}
