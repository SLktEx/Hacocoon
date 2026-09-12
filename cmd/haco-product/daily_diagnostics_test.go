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

func TestSSHFailureRoutesToPolicyInspectionWithoutClaimingCause(t *testing.T) {
	var out bytes.Buffer
	dailyFailure(&out, "open", "ssh_connection", "dev", control.NewStatusError("internal", "SYNTHETIC_PRIVATE_BACKEND"))
	text := out.String()
	for _, want := range []string{"stage=ssh_connection reason=failed", "resource state is unknown", "If the Base lacks sshd", "haco approve --list", "haco config", "No approval or package failure is inferred"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "SYNTHETIC_PRIVATE_BACKEND") {
		t.Fatal("backend output leaked")
	}
	out.Reset()
	dailyFailure(&out, "open", "desktop_client", "dev", control.ErrUnavailable)
	if strings.Contains(out.String(), "package access") {
		t.Fatal("desktop resolution cannot establish an SSH package prerequisite")
	}
}
