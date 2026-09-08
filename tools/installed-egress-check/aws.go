package main

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

type environmentExecutor interface {
	ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error)
}

func checkGuestAWS(ctx context.Context, client environmentExecutor, name string) error {
	// This unique profile is deliberately unconfigured; no AWS credentials or
	// operation allow rule is introduced by acceptance.
	profile := "absent-" + name
	commands := []struct {
		phase    string
		argv     []string
		code     int
		contains string
	}{
		{"entry", []string{"readlink", "/usr/local/bin/haco"}, 0, "/usr/local/libexec/hacocoon-dns"},
		{"source-selection", []string{"haco", "aws", "s3", "ls", "--env", "other", "s3://example-bucket/"}, 2, "--env is unavailable inside an Environment"},
		{"host-authentication-refusal", []string{"haco", "aws", "s3", "ls", "--profile", profile, "s3://example-bucket/"}, 1, "AWS operation did not succeed"},
		{"download-preservation", []string{"/bin/sh", "-ec", guestAWSDownloadProbe, "haco-aws-probe", profile}, 0, "existing-file-preserved"},
	}
	for _, command := range commands {
		result, err := client.ExecEnvironment(ctx, name, command.argv)
		if err != nil {
			return fmt.Errorf("guest AWS %s transport failed", command.phase)
		}
		output := result.Stdout
		if command.code != 0 {
			output = result.Stderr
		}
		if result.ExitCode != command.code || result.StdoutTruncated || result.StderrTruncated || !strings.Contains(output, command.contains) {
			return fmt.Errorf("guest AWS %s acceptance failed (exit %d)", command.phase, result.ExitCode)
		}
	}
	return nil
}

const guestAWSDownloadProbe = `target=$(mktemp /tmp/haco-aws-existing.XXXXXXXX)
diagnostic=$(mktemp /tmp/haco-aws-error.XXXXXXXX)
trap 'rm -f -- "$target" "$diagnostic"' EXIT
printf 'existing-file' > "$target"
set +e
haco aws s3 cp --profile "$1" s3://example-bucket/object "$target" 2>"$diagnostic"
code=$?
set -e
test "$code" -eq 1
grep -q 'AWS operation did not succeed' "$diagnostic"
test "$(cat "$target")" = existing-file
printf 'existing-file-preserved
'
`
