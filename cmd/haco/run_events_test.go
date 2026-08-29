package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseRunSpec(t *testing.T) {
	spec, jsonOutput, err := parseRunSpec([]string{"--read-only", "--workspace", "/work/demo", "--json", "--", "sh", "-c", "echo ok"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.WorkspacePath != "/work/demo" || spec.AccessMode != core.WorkspaceReadOnly || !jsonOutput || !reflect.DeepEqual(spec.Argv, []string{"sh", "-c", "echo ok"}) {
		t.Fatalf("spec=%#v json=%t", spec, jsonOutput)
	}

	defaults, jsonOutput, err := parseRunSpec([]string{"--workspace", "/work/demo", "--", "true"})
	if err != nil || defaults.AccessMode != core.WorkspaceReadWrite || jsonOutput {
		t.Fatalf("defaults=%#v json=%t err=%v", defaults, jsonOutput, err)
	}
}

func TestParseRunSpecRejectsInvalidForms(t *testing.T) {
	for _, args := range [][]string{
		{"--workspace", "/work/demo", "true"},
		{"--workspace", "/work/demo", "--"},
		{"--", "true"},
		{"--workspace", "/a", "--workspace", "/b", "--", "true"},
		{"--workspace", "/a", "--json", "--json", "--", "true"},
		{"--bogus", "--workspace", "/a", "--", "true"},
	} {
		if _, _, err := parseRunSpec(args); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}
