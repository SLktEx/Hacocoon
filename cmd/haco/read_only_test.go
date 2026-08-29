package main

import (
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseCreateSpecReadOnly(t *testing.T) {
	spec, err := parseCreateSpec([]string{"--read-only", "--workspace", "/tmp/work", "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "demo" || spec.WorkspacePath != "/tmp/work" || spec.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestParseCreateSpecDefaultsReadWrite(t *testing.T) {
	spec, err := parseCreateSpec([]string{"--workspace", "/tmp/work", "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.AccessMode != core.WorkspaceReadWrite {
		t.Fatalf("mode = %q", spec.AccessMode)
	}
}
