package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type awsExecFixture struct {
	calls   int
	failure int
	kind    string
}

func (f *awsExecFixture) ExecEnvironment(_ context.Context, _ string, argv []string) (core.ExecutionResult, error) {
	f.calls++
	if f.calls == f.failure {
		if f.kind == "transport" {
			return core.ExecutionResult{}, errors.New("secret diagnostic")
		}
		if f.kind == "truncated" {
			return core.ExecutionResult{StdoutTruncated: true}, nil
		}
		return core.ExecutionResult{ExitCode: 1, Stderr: "unmanaged source"}, nil
	}
	switch f.calls {
	case 1:
		return core.ExecutionResult{Stdout: "/usr/local/libexec/hacocoon-dns"}, nil
	case 2:
		return core.ExecutionResult{ExitCode: 2, Stderr: "--env is unavailable inside an Environment"}, nil
	case 3:
		if argv[5] != "absent-m1-egress-0123456789abcdef" {
			return core.ExecutionResult{}, errors.New("wrong profile")
		}
		return core.ExecutionResult{ExitCode: 1, Stderr: "AWS operation did not succeed"}, nil
	default:
		return core.ExecutionResult{Stdout: "existing-file-preserved"}, nil
	}
}
func TestInstalledGuestAWSRequiresEveryAcceptancePhase(t *testing.T) {
	for failed := 0; failed <= 4; failed++ {
		for _, kind := range []string{"transport", "truncated", "wrong-result"} {
			fixture := &awsExecFixture{failure: failed, kind: kind}
			err := checkGuestAWS(context.Background(), fixture, "m1-egress-0123456789abcdef")
			if failed == 0 && (err != nil || fixture.calls != 4) {
				t.Fatal(err, fixture.calls)
			}
			if failed != 0 && (err == nil || fixture.calls != failed) {
				t.Fatal("failed acceptance passed", failed, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("raw guest error exposed")
			}
		}
	}
}
