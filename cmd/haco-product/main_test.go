package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	}()

	code := run(args)
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	_ = outR.Close()
	_ = errR.Close()
	return code, string(stdout), string(stderr)
}

func TestHelpDoesNotNeedRuntime(t *testing.T) {
	code, stdout, stderr := captureRun(t, "help")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"Hacocoon", "Usage:", "version"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestVersionDoesNotNeedRuntime(t *testing.T) {
	code, stdout, stderr := captureRun(t, "--version")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "haco ") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestUnknownCommandFailsClearly(t *testing.T) {
	code, stdout, stderr := captureRun(t, "env")
	if code != 2 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stderr, `command "env" is not available yet`) {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}
