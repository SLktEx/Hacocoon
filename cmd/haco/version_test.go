package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
)

func TestHandleVersionArgsDetailed(t *testing.T) {
	var out bytes.Buffer
	handled, err := handleVersionArgs([]string{"version"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("version command was not handled")
	}
	for _, want := range []string{"temporary migration CLI", "hacoq", "scheduled for deletion", "checkpoint:", "version:", "commit:", "built:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
}

func TestHandleVersionArgsJSON(t *testing.T) {
	var out bytes.Buffer
	handled, err := handleVersionArgs([]string{"version", "--json"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("version command was not handled")
	}
	var got buildinfo.Info
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, out.String())
	}
	if got.Checkpoint == "" || got.Version == "" || got.Commit == "" || got.BuildDate == "" {
		t.Fatalf("incomplete version payload: %+v", got)
	}
}

func TestHandleVersionArgsCompact(t *testing.T) {
	var out bytes.Buffer
	handled, err := handleVersionArgs([]string{"--version"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !strings.HasPrefix(out.String(), "hacoq ") || !strings.Contains(out.String(), "temporary migration CLI") || !strings.Contains(out.String(), "checkpoint") {
		t.Fatalf("unexpected compact output: %q", out.String())
	}
}

func TestHandleVersionArgsRejectsUnknownOption(t *testing.T) {
	handled, err := handleVersionArgs([]string{"version", "--wat"}, &bytes.Buffer{})
	if !handled || err == nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}
