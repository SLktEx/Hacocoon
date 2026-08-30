package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestAmbiguousRunInstancesRetryReusesClientTokenAcrossRealProcessBoundary(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(root, "run-counter")
	tokens := filepath.Join(root, "tokens")
	logPath := filepath.Join(root, "aws.log")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
args="$*"
case "$args" in
  *" sts get-caller-identity "*) printf '%s\n' '123456789012' ;;
  *" ec2 run-instances "*)
    token=''
    prev=''
    for arg in "$@"; do
      if [ "$prev" = '--client-token' ]; then token="$arg"; break; fi
      prev="$arg"
    done
    [ -n "$token" ] || exit 92
    printf '%s\n' "$token" >> "$HACO_FAKE_AWS_TOKENS"
    n=0; [ -f "$HACO_FAKE_AWS_COUNTER" ] && n="$(cat "$HACO_FAKE_AWS_COUNTER")"
    n=$((n+1)); printf '%s' "$n" > "$HACO_FAKE_AWS_COUNTER"
    if [ "$n" -eq 1 ]; then exit 55; fi
    printf '%s\n' 'i-0123456789abcdef0'
    ;;
  *" ssm describe-instance-information "*) printf '%s\n' 'Online' ;;
  *" ssm send-command "*) printf '%s\n' '11111111-1111-1111-1111-111111111111' ;;
  *" ssm get-command-invocation "*) printf '%s\n' '{"Status":"Success","ResponseCode":0,"StandardOutputContent":"","StandardErrorContent":""}' ;;
  *" s3 cp "*)
    set -- $args
    prev=''
    for arg in "$@"; do
      if [ "$prev" = 'cp' ] && [ "${arg#s3://}" = "$arg" ]; then [ -f "$arg" ] || exit 91; fi
      prev="$arg"
    done
    ;;
esac
`
	aws := filepath.Join(bin, "aws")
	if err := os.WriteFile(aws, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_AWS_LOG", logPath)
	t.Setenv("HACO_FAKE_AWS_COUNTER", counter)
	t.Setenv("HACO_FAKE_AWS_TOKENS", tokens)

	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "host.txt"), []byte("from-host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journalDir := filepath.Join(root, "create-journal")

	first, err := NewWithCreateJournal(host.ExecRunner{}, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	first.pollAttempts = 1
	first.pollDelay = 0
	_, err = first.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "remote", WorkspacePath: workspace, ReadOnly: true})
	if err == nil || errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("ambiguous idempotent create must remain safely retryable, err=%v", err)
	}

	second, err := NewWithCreateJournal(host.ExecRunner{}, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	second.pollAttempts = 1
	second.pollDelay = 0
	created, err := second.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "remote", WorkspacePath: workspace, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(tokens)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(content))
	if len(lines) != 2 || lines[0] != lines[1] || lines[0] != ref.ClientToken {
		t.Fatalf("client token was not stable across process-boundary retries: tokens=%v ref=%#v", lines, ref)
	}
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logContent), "s3 rm s3://hacocoon-workspaces-example/tests/remote") {
		t.Fatalf("ambiguous create removed retry staging:\n%s", logContent)
	}
}
