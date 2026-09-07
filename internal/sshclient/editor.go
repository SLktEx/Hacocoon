//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Editor prepares Remote-SSH in the user's installed desktop client, without interpolating
// Environment names or provider output into PowerShell commands.
func Editor(ctx context.Context, d Desktop) (string, error) {
	if !d.Windows {
		path, err := exec.LookPath("code")
		if err != nil {
			return "", err
		}
		if err = ensureRemoteSSH(ctx, false, path); err != nil {
			return "", err
		}
		return path, nil
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); $c = Get-Command code.cmd -ErrorAction Stop; ConvertTo-Json -Compress (Resolve-Path (Join-Path (Split-Path $c.Source) '../Code.exe')).Path")
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var native string
	if json.Unmarshal(b, &native) != nil || len(native) < 4 || native[1:3] != ":\\" ||
		!((native[0] >= 'A' && native[0] <= 'Z') || (native[0] >= 'a' && native[0] <= 'z')) ||
		strings.ContainsAny(native, "\r\n\x00") {
		return "", fmt.Errorf("invalid VS Code executable path")
	}
	if err = ensureRemoteSSH(ctx, true, ""); err != nil {
		return "", err
	}
	return "/mnt/" + strings.ToLower(native[:1]) + "/" + strings.ReplaceAll(native[3:], "\\", "/"), nil
}

// ensureRemoteSSH is client integration setup, not a Core dependency.
func ensureRemoteSSH(ctx context.Context, windows bool, executable string) error {
	var list *exec.Cmd
	if windows {
		list = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
			"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); & (Get-Command code.cmd -ErrorAction Stop).Source --list-extensions; exit $LASTEXITCODE")
	} else {
		list = exec.CommandContext(ctx, executable, "--list-extensions")
	}
	out, err := list.Output()
	if err != nil {
		return fmt.Errorf("inspect VS Code extensions: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "ms-vscode-remote.remote-ssh") {
			return nil
		}
	}
	var install *exec.Cmd
	if windows {
		install = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
			"& (Get-Command code.cmd -ErrorAction Stop).Source --install-extension ms-vscode-remote.remote-ssh; exit $LASTEXITCODE")
	} else {
		install = exec.CommandContext(ctx, executable, "--install-extension", "ms-vscode-remote.remote-ssh")
	}
	if err := install.Run(); err != nil {
		return fmt.Errorf("install VS Code Remote-SSH: %w", err)
	}
	return nil
}
