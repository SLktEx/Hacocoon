//go:build linux

package sshclient

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This opt-in fixture checks real WSL interop and native Windows client files.
// Its controller is fake: it does not establish provider or SSH transport acceptance.
func TestWindowsDesktopProjectionE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_WINDOWS_DESKTOP") != "1" {
		t.Skip("requires trusted Host Windows interop")
	}
	ctx := context.Background()
	real, err := ResolveDesktop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !real.Windows {
		t.Fatal("not a Windows desktop projection")
	}
	home, err := os.MkdirTemp(filepath.Join(real.Home, "AppData/Local/Temp"), "haco-ssh-client-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	rel, err := filepath.Rel(real.Home, home)
	if err != nil {
		t.Fatal(err)
	}
	desktop := Desktop{Home: home, NativeHome: real.NativeHome + "\\" + strings.ReplaceAll(rel, "/", "\\"), Windows: true}
	c := &fakeController{state: "running"}
	alias, err := Setup(ctx, c, desktop, "projection")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Setup(ctx, c, desktop, "projection"); err != nil {
		t.Fatal(err)
	}
	if c.count != 1 {
		t.Fatal("native setup did not reuse connection")
	}
	// -F selects only this fixture's managed host entry. -G parses configuration
	// without opening a network connection or reading the operator's SSH config.
	config := desktop.NativeHome + "\\.ssh\\hacocoon\\projection.conf"
	out, err := exec.CommandContext(ctx, "ssh.exe", "-G", "-D", "127.0.0.1:49101", "-F", config, alias).Output()
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	for _, want := range []string{"hostname 127.0.0.1\n", "port 23001\n", "stricthostkeychecking true\n", "hostkeyalias haco-projection\n", "dynamicforward [127.0.0.1]:49101\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("native SSH projection missing %q", want)
		}
	}
	t.Log("PASS native Windows key generation, confined client-file writes, reconnect reuse and OpenSSH configuration parsing; no transport exercised")
}

func TestWindowsEditorPreparationE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_WINDOWS_EDITOR") != "1" {
		t.Skip("requires explicit desktop editor preparation acceptance")
	}
	ctx := context.Background()
	d, err := ResolveDesktop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Windows {
		t.Fatal("requires Windows desktop")
	}
	path, err := Editor(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("editor executable: %v", err)
	}
	t.Log("PASS actual Windows VS Code discovery and Remote-SSH extension preparation; editor connection not exercised")
}
