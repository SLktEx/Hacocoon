package awscap_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awscapapp "github.com/SLktEx/Hacocoon/internal/awscap"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type allowPolicy struct{}

func (allowPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyAllow}, nil
}

type audit struct{ events []core.CapabilityAuditEvent }

func (a *audit) Record(_ context.Context, e core.CapabilityAuditEvent) error {
	a.events = append(a.events, e)
	return nil
}

func TestAWSReadCapabilityCrossesRealProcessBoundaryWithoutCredentialArguments(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "aws.log")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
printf '%s\n' '{"Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/test"}'
`
	if err := os.WriteFile(filepath.Join(bin, "aws"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_AWS_LOG", log)

	provider := awscapapp.NewProvider(host.ExecRunner{})
	sink := &audit{}
	service, err := capabilityapp.New(allowPolicy{}, nil, sink, provider)
	if err != nil {
		t.Fatal(err)
	}
	broker := awscapapp.NewBroker(service)
	result, err := broker.Query(context.Background(), awscapapp.QuerySpec{AccountID: "123456789012", Region: "ap-northeast-1", Kind: awscapapp.QueryCallerIdentity})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "123456789012") {
		t.Fatalf("output=%q", result.Output)
	}

	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "sts get-caller-identity") || strings.Contains(strings.ToUpper(text), "SECRET") || strings.Contains(strings.ToUpper(text), "TOKEN") {
		t.Fatalf("aws argv=%s", text)
	}
	if len(sink.events) == 0 || sink.events[0].Resource != "aws://123456789012/ap-northeast-1/identity" {
		t.Fatalf("audit=%#v", sink.events)
	}
}
