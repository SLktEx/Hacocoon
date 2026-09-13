package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteCLIResultHumanReadableByDefault(t *testing.T) {
	value := struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}{Name: "demo", Count: 2}
	var out bytes.Buffer
	if err := writeCLIResult(&out, value, false); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if json.Valid(out.Bytes()) {
		t.Fatalf("default output must not be JSON: %q", text)
	}
	if !strings.Contains(text, "name: demo") || !strings.Contains(text, "count: 2") {
		t.Fatalf("human output missing fields: %q", text)
	}
}

func TestWriteCLIResultJSONRequiresExplicitMode(t *testing.T) {
	value := map[string]any{"name": "demo", "count": 2}
	var out bytes.Buffer
	if err := writeCLIResult(&out, value, true); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("--json output is not valid JSON: %q", out.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || decoded["name"] != "demo" {
		t.Fatalf("unexpected JSON result: %#v err=%v", decoded, err)
	}
}

func TestSplitJSONFlag(t *testing.T) {
	args, machine, err := splitJSONFlag([]string{"list", "--json", "value"})
	if err != nil || !machine || len(args) != 2 || args[0] != "list" || args[1] != "value" {
		t.Fatalf("args=%v machine=%t err=%v", args, machine, err)
	}
	if _, _, err := splitJSONFlag([]string{"--json", "--json"}); err == nil {
		t.Fatal("duplicate --json must be rejected")
	}
}
