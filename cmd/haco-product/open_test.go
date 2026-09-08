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
	if code != 0 || !strings.Contains(err, "desktop client: vscode or ssh") {
		t.Fatalf("%d %s", code, err)
	}
}
