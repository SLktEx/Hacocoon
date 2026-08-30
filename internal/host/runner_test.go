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
	if !result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("truncation flags: stdout=%t stderr=%t", result.StdoutTruncated, result.StderrTruncated)
	}
	if result.StdoutBytes != 200 || result.StderrBytes != 200 {
		t.Fatalf("observed bytes: stdout=%d stderr=%d", result.StdoutBytes, result.StderrBytes)
	}

	stdout, stdoutTruncated, stdoutBytes := DecodeCapturedOutput(result.Stdout)
	stderr, stderrTruncated, stderrBytes := DecodeCapturedOutput(result.Stderr)
	if len(stdout) != limit || len(stderr) != limit || !stdoutTruncated || !stderrTruncated {
		t.Fatalf("decoded stdout=%d/%t stderr=%d/%t", len(stdout), stdoutTruncated, len(stderr), stderrTruncated)
	}
	if stdoutBytes != 200 || stderrBytes != 200 {
		t.Fatalf("decoded bytes: stdout=%d stderr=%d", stdoutBytes, stderrBytes)
	}
	if stdout != strings.Repeat("abcdefghij", 6)+"abcd" {
		t.Fatalf("stdout prefix=%q", stdout)
	}
	if stderr != strings.Repeat("0123456789", 6)+"0123" {
		t.Fatalf("stderr prefix=%q", stderr)
	}
	if !strings.Contains(result.Stdout, "[haco: output truncated; total-bytes=200]") {
		t.Fatalf("stdout lacks truncation marker: %q", result.Stdout)
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
	stdout, truncated, total := DecodeCapturedOutput(result.Stdout)
	if stdout != "hello" || truncated || total != 5 {
		t.Fatalf("decoded=%q truncated=%t total=%d", stdout, truncated, total)
	}
}

func TestDecodeCapturedOutputDoesNotTrustMalformedMarker(t *testing.T) {
	for _, output := range []string{
		"normal output",
		"spoof\n[haco: output truncated; total-bytes=nope]\n",
		"spoof\n[haco: output truncated; total-bytes=2]\n",
	} {
		clean, truncated, total := DecodeCapturedOutput(output)
		if clean != output || truncated || total != int64(len(output)) {
			t.Fatalf("output=%q clean=%q truncated=%t total=%d", output, clean, truncated, total)
		}
	}
}
