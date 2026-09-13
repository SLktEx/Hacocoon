//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type configCLIClient struct {
	edit     capability.PolicySnapshot
	replaces int
	conflict bool
}

func (*configCLIClient) ReadConfiguration(context.Context) (capability.PolicySnapshot, error) {
	return capability.PolicySnapshot{Revision: "sha256:" + strings.Repeat("a", 64), Policy: json.RawMessage(`{"default":"deny","rules":[]}`)}, nil
}
func (c *configCLIClient) ReplaceConfiguration(_ context.Context, edit capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	c.replaces++
	c.edit = edit
	if c.conflict {
		return capability.PolicySnapshot{}, core.ErrIncompatibleState
	}
	edit.Revision = "sha256:" + strings.Repeat("b", 64)
	return edit, nil
}

func TestConfigurationCLIInspectAndFileApply(t *testing.T) {
	client := &configCLIClient{}
	var out, diagnostic bytes.Buffer
	if code := configurationCommand(context.Background(), client, nil, &out, &diagnostic, nil); code != 0 || client.replaces != 0 {
		t.Fatal("inspect changed configuration")
	}
	if json.Valid(out.Bytes()) || !strings.Contains(out.String(), "revision: sha256:") {
		t.Fatal("default configuration output should be human-readable", out.String())
	}

	out.Reset()
	if code := configurationCommand(context.Background(), client, []string{"--json"}, &out, &diagnostic, nil); code != 0 || client.replaces != 0 || !json.Valid(out.Bytes()) {
		t.Fatal("explicit JSON configuration inspect failed", out.String())
	}
	path := filepath.Join(t.TempDir(), "configuration.json")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := configurationCommand(context.Background(), client, []string{"--file", path, "--json"}, &out, &diagnostic, nil); code != 0 || client.replaces != 1 || client.edit.Revision != "sha256:"+strings.Repeat("a", 64) {
		t.Fatal("file revision not preserved")
	}
	client.replaces = 0
	if err := os.WriteFile(path, []byte(`{"revision":"x","policy":{},"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if code := configurationCommand(context.Background(), client, []string{"--file", path}, &out, &diagnostic, nil); code == 0 || client.replaces != 0 {
		t.Fatal("invalid file reached controller")
	}
}

func TestConfigurationCLIConflictRetainsEditorWork(t *testing.T) {
	client := &configCLIClient{conflict: true}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	code := configurationCommand(context.Background(), client, []string{"--edit"}, &out, &diagnostic, func(_ context.Context, s capability.PolicySnapshot) (capability.PolicySnapshot, string, error) {
		return s, path, nil
	})
	if code == 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), path) {
		t.Fatal("conflict was acknowledged or edit location lost")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("conflicting edit was deleted")
	}
}

func TestConfigurationFileRefusesSymlinkHardlinkAndFIFO(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			path := filepath.Join(dir, "input")
			if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "hardlink":
				err = os.Link(target, path)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = readConfigurationFile(path); err == nil {
				t.Fatal("unsafe input read")
			}
		})
	}
}
