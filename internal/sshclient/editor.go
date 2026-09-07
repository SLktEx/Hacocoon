//go:build linux

package sshclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	// WSL captures Windows PATH for the trusted Host, but a native child may
	// retain a different Windows PATH. Use the exact captured CLI when available.
	if cli, err := exec.LookPath("code.cmd"); err == nil {
		nativeCLI, err := projectedWindowsPath(cli)
		if err != nil {
			return "", err
		}
		editor := filepath.Join(filepath.Dir(filepath.Dir(cli)), "Code.exe")
		info, err := os.Stat(editor)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("VS Code executable is missing beside its CLI")
		}
		if err := ensureRemoteSSH(ctx, true, nativeCLI); err != nil {
			return "", err
		}
		return editor, nil
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); $c = Get-Command code.cmd -ErrorAction Stop; ConvertTo-Json -Compress (Resolve-Path (Join-Path (Split-Path $c.Source) '../Code.exe')).Path")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve Windows VS Code executable: %w", err)
	}
	var native string
	if json.Unmarshal(b, &native) != nil || len(native) < 4 || native[1:3] != ":\\" ||
		!((native[0] >= 'A' && native[0] <= 'Z') || (native[0] >= 'a' && native[0] <= 'z')) ||
		strings.ContainsAny(native, "\r\n\x00") {
		return "", fmt.Errorf("invalid VS Code executable path")
	}
	if err = ensureRemoteSSH(ctx, true, strings.TrimSuffix(native, "Code.exe")+"bin\\code.cmd"); err != nil {
		return "", err
	}
	return "/mnt/" + strings.ToLower(native[:1]) + "/" + strings.ReplaceAll(native[3:], "\\", "/"), nil
}

// ensureRemoteSSH is client integration setup, not a Core dependency.
func ensureRemoteSSH(ctx context.Context, windows bool, executable string) error {
	var list *exec.Cmd
	if windows {
		list = windowsEditorCommand(ctx, executable, false)
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
		install = windowsEditorCommand(ctx, executable, true)
	} else {
		install = exec.CommandContext(ctx, executable, "--install-extension", "ms-vscode-remote.remote-ssh")
	}
	if err := install.Run(); err != nil {
		return fmt.Errorf("install VS Code Remote-SSH: %w", err)
	}
	return nil
}

// Encode the selected path as data, never as PowerShell source or shell arguments.
func windowsEditorCommand(ctx context.Context, nativeCLI string, install bool) *exec.Cmd {
	encoded := base64.StdEncoding.EncodeToString([]byte(nativeCLI))
	script := "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); $p = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('" + encoded + "')); & $p "
	if install {
		script += "--install-extension ms-vscode-remote.remote-ssh"
	} else {
		script += "--list-extensions"
	}
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script+"; exit $LASTEXITCODE")
}
func projectedWindowsPath(path string) (string, error) {
	path = filepath.Clean(path)
	if len(path) < 8 || !strings.HasPrefix(path, "/mnt/") || path[5] < 'a' || path[5] > 'z' || path[6] != '/' || strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("VS Code CLI is not on a projected Windows drive")
	}
	return strings.ToUpper(path[5:6]) + ":\\" + strings.ReplaceAll(path[7:], "/", "\\"), nil
}
