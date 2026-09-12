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
}

func TestNoneClientRequiresAPathBeforeControllerSetup(t *testing.T) {
	code, _, diagnostic := captureRun(t, "open", "--client", "none", "missing-env-name")
	if code != 2 || !strings.Contains(diagnostic, "require a directory") {
		t.Fatal(code, diagnostic)
	}
}
