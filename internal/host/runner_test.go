package host

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunnerBoundsStdoutAndStderrWithoutStoppingChild(t *testing.T) {
	const limit = 64
	runner := ExecRunner{MaxOutputBytes: limit}
	result, err := runner.Run(context.Background(), "sh", "-c", `
		i=0
		while [ "$i" -lt 20 ]; do
			printf 'abcdefghij'
			printf '0123456789' >&2
			i=$((i + 1))
		done
		exit 7
	`)
	if err == nil {
		t.Fatal("expected exit error")
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit=%d", result.ExitCode)
	}
	if len(result.Stdout) != limit || len(result.Stderr) != limit {
		t.Fatalf("captured stdout=%d stderr=%d", len(result.Stdout), len(result.Stderr))
	}
	if !result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("truncation flags: stdout=%t stderr=%t", result.StdoutTruncated, result.StderrTruncated)
	}
	if result.StdoutBytes != 200 || result.StderrBytes != 200 {
		t.Fatalf("observed bytes: stdout=%d stderr=%d", result.StdoutBytes, result.StderrBytes)
	}
	if result.Stdout != strings.Repeat("abcdefghij", 6)+"abcd" {
		t.Fatalf("stdout prefix=%q", result.Stdout)
	}
	if result.Stderr != strings.Repeat("0123456789", 6)+"0123" {
		t.Fatalf("stderr prefix=%q", result.Stderr)
	}
}

func TestExecRunnerPreservesSmallOutputMetadata(t *testing.T) {
	runner := ExecRunner{MaxOutputBytes: 64}
	result, err := runner.Run(context.Background(), "sh", "-c", `printf 'hello'; printf 'warn' >&2`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "hello" || result.Stderr != "warn" || result.StdoutTruncated || result.StderrTruncated {
		t.Fatalf("result=%#v", result)
	}
	if result.StdoutBytes != 5 || result.StderrBytes != 4 {
		t.Fatalf("result=%#v", result)
	}
}
