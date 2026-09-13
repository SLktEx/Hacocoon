package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type recoveryCLIStub struct {
	request   controlapi.GitStatusRequest
	reconcile bool
	calls     int
	err       error
	result    gitrepo.PushStatus
}

func (s *recoveryCLIStub) GitPushStatus(_ context.Context, request controlapi.GitStatusRequest, reconcile bool) (gitrepo.PushStatus, error) {
	s.request, s.reconcile = request, reconcile
	s.calls++
	return s.result, s.err
}

func TestGitRecoveryCLIExactReadSelectionAndBilingualUncertainty(t *testing.T) {
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		setCLITestLocale(t, locale)
		for _, op := range []string{"status", "reconcile"} {
			stub := &recoveryCLIStub{result: gitrepo.PushStatus{Found: true, Environment: "dev", Repository: "source", Ref: "refs/heads/main", State: "unconfirmed", Observation: "matches-new"}}
			var out, diagnostic bytes.Buffer
			code := gitRecoveryWithClient(context.Background(), stub, []string{op, "--request", "request-1", "dev"}, &out, &diagnostic)
			if code != 0 || diagnostic.Len() != 0 || stub.calls != 1 || stub.reconcile != (op == "reconcile") || stub.request.RequestID != "request-1" || stub.request.Environment != "dev" {
				t.Fatalf("%d %s %+v", code, diagnostic.String(), stub)
			}
			for _, key := range []string{"git.recovery.unconfirmed", "git.recovery.matches-new", "git.recovery.next"} {
				if !strings.Contains(out.String(), cliMessage(key)) {
					t.Fatalf("missing %s: %s", key, out.String())
				}
			}
			out.Reset()
			if code := gitRecoveryWithClient(context.Background(), stub, []string{op, "--json", "dev"}, &out, &diagnostic); code != 0 {
				t.Fatal(code)
			}
			var decoded gitrepo.PushStatus
			if json.Unmarshal(out.Bytes(), &decoded) != nil || decoded != stub.result {
				t.Fatal(out.String())
			}
		}
	}
}

func TestGitRecoveryCLIFailureDoesNotPrintSuccessOrRetry(t *testing.T) {
	stub := &recoveryCLIStub{err: errors.New("recovery-required"), result: gitrepo.PushStatus{Found: true, State: "confirmed"}}
	var out, diagnostic bytes.Buffer
	if code := gitRecoveryWithClient(context.Background(), stub, []string{"reconcile", "dev"}, &out, &diagnostic); code != 1 || stub.calls != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "recovery-required") {
		t.Fatalf("%d %s %s", code, out.String(), diagnostic.String())
	}
	for _, args := range [][]string{{"reconcile"}, {"reconcile", "--force", "dev"}, {"status", "dev", "extra"}} {
		if code := gitRecoveryWithClient(context.Background(), stub, args, &out, &diagnostic); code != 2 || stub.calls != 1 {
			t.Fatalf("%v %d", args, code)
		}
	}
}
