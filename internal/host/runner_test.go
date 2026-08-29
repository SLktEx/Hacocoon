package host

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunnerBoundsCapturedOutput(t *testing.T) {
	runner := ExecRunner{OutputLimit: 32}
	result, err := runner.Run(context.Background(), "sh", "-c", "printf '%050d' 0; printf '%050d' 0 >&2")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Stdout) != 32 || len(result.Stderr) != 32 {
		t.Fatalf("stdout=%d stderr=%d", len(result.Stdout), len(result.Stderr))
	}
	if !result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("truncation flags: stdout=%v stderr=%v", result.StdoutTruncated, result.StderrTruncated)
	}
	if strings.Contains(result.Stdout, "00000000000000000000000000000000000000000000000000") {
		t.Fatal("stdout was not bounded")
	}
}
