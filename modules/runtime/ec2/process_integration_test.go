package ec2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEC2RuntimeCrossesRealAWSCLIProcessBoundary(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "aws.log")
	counter := filepath.Join(root, "counter")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
args="$*"
case "$args" in
  *" sts get-caller-identity "*) printf '%s\n' '123456789012' ;;
  *" ssm send-command "*)
    n=0; [ -f "$HACO_FAKE_AWS_COUNTER" ] && n="$(cat "$HACO_FAKE_AWS_COUNTER")"
    n=$((n+1)); printf '%s' "$n" > "$HACO_FAKE_AWS_COUNTER"
    if [ "$n" -eq 1 ]; then printf '%s\n' '11111111-1111-1111-1111-111111111111'; else printf '%s\n' '22222222-2222-2222-2222-222222222222'; fi
    ;;
  *" ec2 run-instances "*) printf '%s\n' 'i-0123456789abcdef0' ;;
  *" ssm describe-instance-information "*) printf '%s\n' 'Online' ;;
  *" s3 cp "*)
    set -- $args
    prev=''
    for arg in "$@"; do
      if [ "$prev" = 'cp' ] && [ "${arg#s3://}" = "$arg" ]; then [ -f "$arg" ] || exit 91; fi
      prev="$arg"
    done
    ;;
  *" ssm get-command-invocation "*)
    case "$args" in
      *22222222-2222-2222-2222-222222222222*) printf '%s\n' '{"Status":"Failed","ResponseCode":17,"StandardOutputContent":"remote-out\n","StandardErrorContent":"remote-err\n"}' ;;
      *) printf '%s\n' '{"Status":"Success","ResponseCode":0,"StandardOutputContent":"","StandardErrorContent":""}' ;;
    esac
    ;;
  *" ec2 describe-instances "*) printf '%s\n' 'running' ;;
esac
`
	aws := filepath.Join(bin, "aws")
	if err := os.WriteFile(aws, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_AWS_LOG", logPath)
	t.Setenv("HACO_FAKE_AWS_COUNTER", counter)

	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "host.txt"), []byte("from-host\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runtime := newTestRuntime(host.ExecRunner{})
	created, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "remote", WorkspacePath: workspace, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil || ref.AccountID != testAWSAccountID || ref.Region != "ap-northeast-1" {
		t.Fatalf("ref=%#v err=%v", ref, err)
	}
	result, err := runtime.ExecEnvironment(context.Background(), created.Ref, core.ExecutionRequest{Argv: []string{"sh", "-c", "printf ok"}})
	if err != nil || result.ExitCode != 17 || result.Stdout != "remote-out\n" || result.Stderr != "remote-err\n" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if err := runtime.DeleteEnvironment(context.Background(), created.Ref); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(content)
	for _, want := range []string{"sts get-caller-identity --query Account --output text", "ec2 run-instances", "--metadata-options HttpTokens=required,HttpEndpoint=enabled", "ec2 wait instance-status-ok", "ssm describe-instance-information", "ssm send-command", "ssm get-command-invocation", "ec2 terminate-instances", "s3 rm"} {
		if !strings.Contains(log, want) {
			t.Fatalf("missing %q in:\n%s", want, log)
		}
	}
	if strings.Contains(strings.ToUpper(log), "AWS_SECRET_ACCESS_KEY") || strings.Contains(strings.ToUpper(log), "AWS_SESSION_TOKEN") {
		t.Fatalf("credentials appeared in process argv:\n%s", log)
	}
}
