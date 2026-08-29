package host

import (
	"context"
	"errors"
	"testing"
)

func TestExecRunnerBoundsCapturedOutput(t *testing.T) {
	result, err := runWithCaptureLimit(context.Background(), 4, "sh", "-c", "printf 1234567890; printf abcdef >&2")
	if !errors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("err=%v", err)
	}
	if result.Stdout != "1234" || result.Stderr != "abcd" {
		t.Fatalf("stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit=%d", result.ExitCode)
	}
}

func TestExecRunnerPreservesExitErrorWhenOutputIsTruncated(t *testing.T) {
	result, err := runWithCaptureLimit(context.Background(), 4, "sh", "-c", "printf 123456; exit 7")
	if !errors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("missing output limit error: %v", err)
	}
	if result.ExitCode != 7 || result.Stdout != "1234" {
		t.Fatalf("result=%#v", result)
	}
}
