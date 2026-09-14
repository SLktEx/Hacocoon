//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestOpenClientChoiceIsExplicitAndValidatedBeforeSetup(t *testing.T) {
	code, _, err := captureRun(t, "open", "--client", "unknown", "dev")
	if code != 2 || !strings.Contains(err, "--client vscode|ssh") {
		t.Fatalf("%d %s", code, err)
	}
	code, out, err := captureRun(t, "open", "--help")
	if code != 0 || err != "" || !strings.Contains(out, "--client vscode|ssh|none") {
		t.Fatalf("%d %s %s", code, out, err)
	}
	if !strings.Contains(out, "environment-or-directory") || !strings.Contains(out, "--repo") || strings.Contains(out, "path discovery is not implemented") {
		t.Fatalf("help does not describe the integrated Workspace entry: %s", out)
	}
}

func TestNoneClientRequiresAPathBeforeControllerSetup(t *testing.T) {
	code, _, diagnostic := captureRun(t, "open", "--client", "none", "missing-env-name")
	if code != 2 || !strings.Contains(diagnostic, "require a directory") {
		t.Fatal(code, diagnostic)
	}
}
