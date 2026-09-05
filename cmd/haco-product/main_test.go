package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
)

func TestLoginWaitsForControllerStartup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	err := waitForController(ctx, func(context.Context) error {
		calls++
		if calls == 1 {
			return control.ErrUnavailable
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

func TestLoginDoesNotRetryProtocolRejection(t *testing.T) {
	calls := 0
	err := waitForController(context.Background(), func(context.Context) error {
		calls++
		return control.ErrProtocol
	})
	if !errors.Is(err, control.ErrProtocol) || calls != 1 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

func TestLoginControllerWaitIsBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := waitForController(ctx, func(context.Context) error { return control.ErrUnavailable })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
}

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

func TestLoginAliasDoesNotTreatDevNullAsInteractive(t *testing.T) {
	file, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = file, file
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
	if stdioIsInteractive() {
		t.Fatal("non-terminal character devices must not enter haco-host")
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

func TestLegacyHostCommandsAreUnavailable(t *testing.T) {
	for _, operation := range []string{"ensure", "shell"} {
		t.Run(operation, func(t *testing.T) {
			code, stdout, stderr := captureRun(t, "host", operation)
			if code != 2 || stdout != "" || !strings.Contains(stderr, `command "host" is not available yet`) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestLoginAliasDetection(t *testing.T) {
	for _, path := range []string{"hacocoon-login", "/usr/local/libexec/hacocoon-login", "-hacocoon-login"} {
		if !isLoginAlias(path) {
			t.Fatalf("expected login alias for %q", path)
		}
	}
	if isLoginAlias("haco") {
		t.Fatal("normal haco must not be treated as the login alias")
	}
}
