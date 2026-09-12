package main

import (
	"bytes"
	"github.com/SLktEx/Hacocoon/internal/control"
	"os"
	"strings"
	"testing"
)

func TestDailyFailureSuppressesBackendAndUnsafeTarget(t *testing.T) {
	var out bytes.Buffer
	dailyFailure(&out, "environment_create", "controller", "bad\nSECRET", control.NewStatusError("recovery_required", "SECRET-backend"))
	if strings.Contains(out.String(), "SECRET") || !strings.Contains(out.String(), "reason=recovery_required") || !strings.Contains(out.String(), "unknown") {
		t.Fatal(out.String())
	}
}
func TestConfirmationDoesNotReadOpenPipe(t *testing.T) {
	in, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer out.Close()
	var diagnostics bytes.Buffer
	if requireInteractiveConfirmation(in, &diagnostics) || !strings.Contains(diagnostics.String(), "--yes") {
		t.Fatal(diagnostics.String())
	}
}
