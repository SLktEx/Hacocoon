package gitcap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestBrokeredGitFailureDoesNotPersistStderrInAudit(t *testing.T) {
	const secret = "ghp_SUPERSECRET_FROM_GIT_STDERR"
	runner := &runnerFunc{fn: func(_ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		if result, err, ok := noLocalSecurityConfig(joined); ok {
			return result, err
		}
		switch {
		case strings.Contains(joined, "check-ref-format --branch main"):
			return host.Result{}, nil
		case strings.Contains(joined, "config --get remote.origin.url"):
			return host.Result{Stdout: "https://github.com/acme/demo.git\n"}, nil
		case strings.Contains(joined, "rev-parse --verify HEAD^{commit}"):
			return host.Result{Stdout: shaA + "\n"}, nil
		case strings.Contains(joined, "cat-file -e "+shaA+"^{commit}"):
			return host.Result{}, nil
		case strings.Contains(joined, "push --porcelain --no-verify https://github.com/acme/demo.git "+shaA+":refs/heads/main"):
			return host.Result{Stderr: "fatal: Authorization: Bearer " + secret + " /home/operator/.ssh/id_ed25519\n", ExitCode: 128}, errors.New("git exited 128")
		default:
			return host.Result{}, fmt.Errorf("unexpected git call: %s", joined)
		}
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work/demo"}}}
	provider := NewProvider(runner, store)
	audit := &auditSink{}
	broker := NewBroker(runner, store, newCaps(t, provider, audit))

	result, err := broker.Push(context.Background(), PushSpec{Environment: "demo", Branch: "main"})
	if err == nil || !strings.Contains(err.Error(), secret) {
		t.Fatalf("immediate Git error should retain diagnostic detail: result=%#v err=%v", result, err)
	}
	if result.ExecutionState != core.CapabilityFailed || !result.AuditComplete {
		t.Fatalf("unexpected result=%#v", result)
	}
	if len(audit.events) != 3 {
		t.Fatalf("events=%#v", audit.events)
	}
	completed := audit.events[2]
	if completed.Type != "completed" || completed.Reason != "provider-execution-failed" {
		t.Fatalf("completed=%#v", completed)
	}
	serialized := fmt.Sprintf("%#v", audit.events)
	for _, forbidden := range []string{secret, "Authorization: Bearer", "/home/operator/.ssh/id_ed25519"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("Git stderr leaked into capability audit: %s", serialized)
		}
	}
}
