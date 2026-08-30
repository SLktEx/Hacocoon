package main

import (
	"fmt"
	"os"
	"os/exec"
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

	hacoBinary, err := os.Executable()
	if err != nil {
		return wslLoginRecoveryError(fmt.Errorf("resolve haco executable: %w", err))
	}
	if resolved, resolveErr := filepath.EvalSymlinks(hacoBinary); resolveErr == nil {
		hacoBinary = resolved
	}
	sudoBinary, err := exec.LookPath("sudo")
	if err != nil {
		return wslLoginRecoveryError(fmt.Errorf("sudo is unavailable: %w", err))
	}
	if err := syscall.Exec(sudoBinary, []string{"sudo", "-n", hacoBinary, "host", "shell"}, os.Environ()); err != nil {
		return wslLoginRecoveryError(fmt.Errorf("enter haco-host: %w", err))
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
