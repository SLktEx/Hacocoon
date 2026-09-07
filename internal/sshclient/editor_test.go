//go:build linux

package sshclient

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorInstallsRemoteSSHOnlyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "code")
	body := "#!/bin/sh\ncd -- \"$(dirname -- \"$0\")\" || exit 1\ncase \"$1\" in\n--list-extensions) if test -f installed; then echo ms-vscode-remote.remote-ssh; fi;;\n--install-extension) test \"$2\" = ms-vscode-remote.remote-ssh || exit 2; test ! -f installed || exit 3; touch installed;;\n*) exit 4;;\nesac\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := ensureRemoteSSH(context.Background(), false, script); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "installed")); err != nil {
		t.Fatal(err)
	}
}

func TestProjectedEditorPathAndCommandData(t *testing.T) {
	path := "/mnt/d/Editors/Code ' & $()/bin/code.cmd"
	native, err := projectedWindowsPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if native != "D:\\Editors\\Code ' & $()\\bin\\code.cmd" {
		t.Fatalf("native = %q", native)
	}
	for _, path := range []string{"/usr/bin/code.cmd", "/mnt/cx/bin/code.cmd", "/mnt/c/bin/\ncode.cmd"} {
		if _, err := projectedWindowsPath(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	command := windowsEditorCommand(context.Background(), native, false)
	script := command.Args[len(command.Args)-1]
	encoded := base64.StdEncoding.EncodeToString([]byte(native))
	if strings.Contains(script, native) || !strings.Contains(script, "FromBase64String('"+encoded+"')") ||
		!strings.Contains(script, "& $p --list-extensions") || strings.Contains(script, "Get-Command") {
		t.Fatal("selected editor path was not passed as data")
	}
}
