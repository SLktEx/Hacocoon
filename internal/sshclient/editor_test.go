//go:build linux

package sshclient

import (
	"context"
	"os"
	"path/filepath"
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
