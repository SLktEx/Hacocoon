//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/experimental"
)

func TestClientOutputLimitAlsoAppliesToIOCopy(t *testing.T) {
	var out boundedOutput
	// exec.Cmd copies process pipes with io.Copy; exposing bytes.Buffer's
	// ReaderFrom would bypass the Write limit even if direct writes were bounded.
	if _, err := io.Copy(&out, io.LimitReader(strings.NewReader(strings.Repeat("x", 9000)), 9000)); err == nil {
		t.Fatal("response limit bypassed")
	}
	if len(out.Bytes()) > 8192 {
		t.Fatal("unbounded response retained")
	}
}

func TestOpenVSCodeAppliesOnlySubtreeOverSSH(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("HACO_TEST_PLAN", filepath.Join(dir, "plan.json"))
	code := "#!/bin/sh\ncase \"$1\" in\n--list-extensions) echo ms-vscode-remote.remote-ssh;;\n--version) printf '1.100.0\\n" + strings.Repeat("a", 40) + "\\nx64\\n';;\n*) exit 5;;\nesac\n"
	ssh := "#!/bin/sh\nfor arg; do command=$arg; done\nif test \"$command\" = 'uname -m'; then echo x86_64; else cat > \"$HACO_TEST_PLAN\"; echo haco-vscode-applied; fi\n"
	for name, body := range map[string]string{"code": code, "ssh": ssh} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	s, err := experimental.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, _ := s.Read(ctx)
	object := map[string]any{"settings": map[string]any{"editor.formatOnSave": true, "test.data": "$(touch /tmp/not-code)"}}
	if err := s.Replace(ctx, snap.Revision, object); err != nil {
		t.Fatal(err)
	}
	launches := 0
	launch := func(editor string) error {
		if editor != filepath.Join(bin, "code") {
			t.Fatal(editor)
		}
		launches++
		return nil
	}
	if err := OpenVSCode(ctx, Desktop{}, "haco-test", launch); err != nil {
		t.Fatal(err)
	}
	if launches != 1 {
		t.Fatal("launch count")
	}
	b, err := os.ReadFile(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if json.Unmarshal(b, &plan) != nil || len(plan) != 2 || plan["settings"].(map[string]any)["test.data"] != "$(touch /tmp/not-code)" {
		t.Fatal("subtree data was not preserved")
	}
	if err := OpenVSCode(ctx, Desktop{}, "--evil", launch); err == nil || launches != 1 {
		t.Fatal("invalid alias reached launch")
	}
	if err := os.Remove(s.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "plan.json")); err != nil {
		t.Fatal(err)
	}
	if err := OpenVSCode(ctx, Desktop{}, "haco-test", launch); err != nil || launches != 2 {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plan.json")); !os.IsNotExist(err) {
		t.Fatal("absent feature invoked guest preparation")
	}
}
