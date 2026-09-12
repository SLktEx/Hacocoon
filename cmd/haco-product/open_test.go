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
	code, _, err = captureRun(t, "open", "--help")
	if code != 0 || !strings.Contains(err, "desktop client: vscode, ssh or none") {
		t.Fatalf("%d %s", code, err)
	}
	if !strings.Contains(err, "<directory>") || !strings.Contains(err, "--repo") || strings.Contains(err, "path discovery is not implemented") {
		t.Fatalf("help does not describe the integrated Workspace entry: %s", err)
	}
}

func TestNoneClientRequiresAPathBeforeControllerSetup(t *testing.T) {
	code, _, diagnostic := captureRun(t, "open", "--client", "none", "missing-env-name")
	if code != 2 || !strings.Contains(diagnostic, "require a directory") {
		t.Fatal(code, diagnostic)
	}
}
