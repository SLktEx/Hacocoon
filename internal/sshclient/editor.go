//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Editor resolves the user's installed desktop client, without interpolating
// Environment names or provider output into PowerShell commands.
func Editor(ctx context.Context, d Desktop) (string, error) {
	if !d.Windows {
		return exec.LookPath("code")
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
	return "/mnt/" + strings.ToLower(native[:1]) + "/" + strings.ReplaceAll(native[3:], "\\", "/"), nil
}
