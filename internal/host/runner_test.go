package host

import (
	"context"
	"errors"
	"os/exec"
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

func TestExecRunnerFailureIncludesCommandAndCapturedOutput(t *testing.T) {
	runner := ExecRunner{MaxOutputBytes: 1024}
	_, err := runner.Run(context.Background(), "sh", "-c", `printf 'normal stdout'; printf 'useful stderr' >&2; exit 9`)
	if err == nil {
		t.Fatal("expected exit error")
	}
	message := err.Error()
	for _, want := range []string{
		"host command failed: sh -c",
		"exit code: 9",
		"stdout:\nnormal stdout",
		"stderr:\nuseful stderr",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure diagnostics missing %q:\n%s", want, message)
		}
	}

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 9 {
		t.Fatalf("underlying exec.ExitError was not preserved: %v", err)
	}
}

func TestExecRunnerFailureDiagnosticsRedactCommandAndOutput(t *testing.T) {
	const argSecret = "arg-secret-value"
	const outputSecret = "output-secret-value"
	runner := ExecRunner{MaxOutputBytes: 1024}
	_, err := runner.Run(
		context.Background(),
		"sh",
		"-c", `printf 'token=output-secret-value' >&2; exit 3`,
		"sh",
		"--token="+argSecret,
	)
	if err == nil {
		t.Fatal("expected exit error")
	}
	message := err.Error()
	for _, secret := range []string{argSecret, outputSecret} {
		if strings.Contains(message, secret) {
			t.Fatalf("secret leaked in failure diagnostics: %s", message)
		}
	}
	if !strings.Contains(message, "--token=[REDACTED]") || !strings.Contains(message, "token=[REDACTED]") {
		t.Fatalf("redaction marker missing from failure diagnostics: %s", message)
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
