//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/experimental"
)

func TestExperimentalStalledJSONInputCanBeCanceled(t *testing.T) {
	s := experimental.Store{Path: filepath.Join(t.TempDir(), "managed", "config.yaml")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, w := io.Pipe()
	defer w.Close()
	done := make(chan int, 1)
	go func() {
		var out, diag bytes.Buffer
		done <- experimentalCommand(ctx, s, []string{"edit", "vscode", "--json", "-"}, r, &out, &diag, nil)
	}()
	if _, err := w.Write([]byte(`{"settings":{}}`)); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case code := <-done:
		if code != 1 {
			t.Fatal(code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled command still waits for stdin EOF")
	}
	if _, err := os.Stat(s.Path); !os.IsNotExist(err) {
		t.Fatal("cancellation wrote configuration")
	}
}

func TestExperimentalSubtreeModes(t *testing.T) {
	dir := t.TempDir()
	s := experimental.Store{Path: filepath.Join(dir, "managed", "config.yaml")}
	ctx := context.Background()
	var out, diagnostic bytes.Buffer
	edit := func(_ context.Context, b []byte) ([]byte, string, error) {
		if strings.Contains(string(b), "experimental:") {
			t.Fatal("editor received whole config")
		}
		return []byte("settings: {editor.formatOnSave: true}"), "", nil
	}
	run := func(args []string, stdin string) int {
		out.Reset()
		diagnostic.Reset()
		return experimentalCommand(ctx, s, append([]string{"edit", "vscode"}, args...), strings.NewReader(stdin), &out, &diagnostic, edit)
	}
	if code := run(nil, ""); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	if code := run([]string{"--json"}, "ignored"); code != 0 {
		t.Fatal(code)
	}
	var object map[string]any
	if err := json.Unmarshal(out.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	object["settings"].(map[string]any)["files.trimTrailingWhitespace"] = true
	b, _ := json.Marshal(object)
	if code := run([]string{"--json", "-"}, string(b)); code != 0 || out.Len() != 0 {
		t.Fatal(code, out.String(), diagnostic.String())
	}
	file := filepath.Join(dir, "vscode.yaml")
	if err := os.WriteFile(file, []byte("extensions: {minReleaseAge: 7d, preRelease: deny}"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"--file", file}, ""); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	snap, err := s.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Object["settings"] != nil {
		t.Fatal("subtree replacement was unexpectedly merged")
	}
	for _, args := range [][]string{{"--json", "bad"}, {"--json", "-", "extra"}, {"--json", "--file", file}, {"-"}} {
		if code := run(args, "{}"); code != 2 {
			t.Fatalf("%v: %d", args, code)
		}
	}
	for _, input := range []string{"null", "settings: {}", `{"settings": {}, "settings": {}}`, `{"extensions":{"force":true}}`, "{} {}"} {
		if code := run([]string{"--json", "-"}, input); code != 1 {
			t.Fatalf("%s: %d", input, code)
		}
	}
	after, _ := s.Read(ctx)
	if after.Revision != snap.Revision {
		t.Fatal("invalid input changed configuration")
	}
}

func TestExperimentalEditorConflictAndRetention(t *testing.T) {
	s := experimental.Store{Path: filepath.Join(t.TempDir(), "managed", "config.yaml")}
	ctx := context.Background()
	var out, diag bytes.Buffer
	edit := func(context.Context, []byte) ([]byte, string, error) {
		snap, _ := s.Read(ctx)
		if err := s.Replace(ctx, snap.Revision, map[string]any{}); err != nil {
			return nil, "", err
		}
		return []byte("settings: {}"), "/tmp/retained/vscode.yaml", nil
	}
	if code := experimentalCommand(ctx, s, []string{"edit", "vscode"}, nil, &out, &diag, edit); code != 1 || !strings.Contains(diag.String(), "/tmp/retained/vscode.yaml") {
		t.Fatal(code, diag.String())
	}
	edit = func(context.Context, []byte) ([]byte, string, error) {
		return nil, "/tmp/retained/vscode.yaml", fmt.Errorf("editor failed")
	}
	if code := experimentalCommand(ctx, s, []string{"edit", "vscode"}, nil, &out, &diag, edit); code != 1 {
		t.Fatal(code)
	}
}

func TestExperimentalRealEditorRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "editor")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat > \"$1\" <<'YAML'\nsettings:\n  editor.formatOnSave: true\nYAML\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", path)
	s := experimental.Store{Path: filepath.Join(dir, "config", "config.yaml")}
	var out, diagnostic bytes.Buffer
	if code := experimentalCommand(context.Background(), s, []string{"edit", "vscode"}, nil, &out, &diagnostic, editVSCode); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	snap, err := s.Read(context.Background())
	if err != nil || snap.Object["settings"].(map[string]any)["editor.formatOnSave"] != true {
		t.Fatal(snap, err)
	}
}
