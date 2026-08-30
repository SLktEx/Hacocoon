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
	return strings.TrimPrefix(filepath.Base(argv0), "-") == hacocoonLoginName
}

func runHacocoonLogin(args []string) error {
	if len(args) > 0 || !stdioIsInteractive() {
		return execLoginShell(args)
	}

	hacoBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve haco executable: %w\n%s", err, physicalHostRecoveryMessage())
	}
	hacoBinary, err = filepath.EvalSymlinks(hacoBinary)
	if err != nil {
		return fmt.Errorf("resolve haco executable symlink: %w\n%s", err, physicalHostRecoveryMessage())
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("automatic haco-host entry requires sudo: %w\n%s", err, physicalHostRecoveryMessage())
	}
	argv := []string{"sudo", "-n", hacoBinary, "host", "shell"}
	if err := syscall.Exec(sudo, argv, os.Environ()); err != nil {
		return fmt.Errorf("enter haco-host: %w\n%s", err, physicalHostRecoveryMessage())
	}
	return nil
}

func execLoginShell(args []string) error {
	shell := "/bin/bash"
	argv := append([]string{"bash"}, args...)
	if err := syscall.Exec(shell, argv, os.Environ()); err != nil {
		return fmt.Errorf("execute Physical Host shell: %w", err)
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

func physicalHostRecoveryMessage() string {
	distro := strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME"))
	if distro == "" {
		distro = "Hacocoon"
	}
	return fmt.Sprintf("Physical Host recovery: wsl -d %s -u root", distro)
}
