//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestConfigurationLanguagePreservesJSONAndPolicy(t *testing.T) {
	var previous [2]string
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		client := &configCLIClient{}
		var out, diagnostic bytes.Buffer
		if code := configurationCommand(context.Background(), client, []string{"--json"}, &out, &diagnostic, nil); code != 0 || client.replaces != 0 {
			t.Fatal("inspection mutated configuration", code)
		}
		path := filepath.Join(t.TempDir(), "configuration.json")
		if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		observed := [2]string{out.String(), ""}
		out.Reset()
		if code := configurationCommand(context.Background(), client, []string{"--file", path, "--json"}, &out, &diagnostic, nil); code != 0 || client.replaces != 1 {
			t.Fatal("apply changed execution count", code)
		}
		observed[1] = out.String()
		if !json.Valid(out.Bytes()) || diagnostic.Len() != 0 || string(client.edit.Policy) != `{"default":"deny","rules":[]}` || client.edit.Revision != "sha256:"+strings.Repeat("a", 64) {
			t.Fatal("presentation changed Policy or the revision-bound request")
		}
		if language == "ja" && observed != previous {
			t.Fatal("JSON changed with language")
		}
		previous = observed
	}
}

func TestConfigurationJapaneseGuidanceAndRetainedConflict(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	client := &configCLIClient{}
	var out, diagnostic bytes.Buffer
	if code := configurationCommand(context.Background(), client, nil, &out, &diagnostic, nil); code != 0 || !strings.Contains(out.String(), "haco config --edit") || !strings.Contains(out.String(), "新たな許可は与えません") {
		t.Fatal("missing inspection guidance", code, out.String())
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	data := []byte(`{"default":"deny","rules":[]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	client.conflict = true
	out.Reset()
	code := configurationCommand(context.Background(), client, []string{"--edit"}, &out, &diagnostic, func(_ context.Context, snapshot capability.PolicySnapshot) (capability.PolicySnapshot, string, error) {
		return snapshot, path, nil
	})
	if code != 1 || client.replaces != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "再実行する前にhaco config") || !strings.Contains(diagnostic.String(), "編集した設定を残しています") || !strings.Contains(diagnostic.String(), core.ErrIncompatibleState.Error()) {
		t.Fatal("conflict lost its original error or was replayed", code, diagnostic.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("conflicting editor work was changed", err)
	}
}

type configurationBrokenWriter struct{}

func (configurationBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestConfigurationSavedGuidanceAndWriteFailureNeverReplay(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		client := &configCLIClient{}
		snapshot, err := client.ReadConfiguration(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "configuration.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		args := []string{"--file", path}
		if code := configurationCommand(context.Background(), client, args, &out, &diagnostic, nil); code != 0 || client.replaces != 1 || !strings.Contains(out.String(), cliMessage("config.saved")) {
			t.Fatal("missing saved result", code, out.String())
		}
		client.replaces = 0
		if code := configurationCommand(context.Background(), client, args, configurationBrokenWriter{}, &diagnostic, nil); code != 1 || client.replaces != 1 {
			t.Fatal("write failure replayed configuration", code)
		}
	}
}
