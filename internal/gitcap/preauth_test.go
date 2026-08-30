package gitcap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type denyGitPolicy struct{}

func (denyGitPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyDeny, Reason: "test deny"}, nil
}

func TestForcePushDoesNotContactRemoteBeforePolicy(t *testing.T) {
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
			return host.Result{Stdout: shaB + "\n"}, nil
		case strings.Contains(joined, "rev-parse --verify refs/remotes/origin/main^{commit}"):
			return host.Result{Stdout: shaA + "\n"}, nil
		case strings.Contains(joined, "ls-remote"):
			return host.Result{}, fmt.Errorf("remote network access occurred before policy")
		default:
			return host.Result{}, fmt.Errorf("unexpected git call: %s", joined)
		}
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work/demo"}}}
	provider := NewProvider(runner, store)
	caps, err := capabilityapp.New(denyGitPolicy{}, noApproval{}, &auditSink{}, provider)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewBroker(runner, store, caps).Push(context.Background(), PushSpec{Environment: "demo", Branch: "main", Force: true})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("expected policy denial before network access, got err=%v calls=%v", err, runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "ls-remote") || strings.Contains(call, " push ") {
			t.Fatalf("privileged Git network operation happened before policy allowed it: %s", call)
		}
	}
}

func TestForcePushRequiresFetchedTrackingRef(t *testing.T) {
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
			return host.Result{Stdout: shaB + "\n"}, nil
		case strings.Contains(joined, "rev-parse --verify refs/remotes/origin/main^{commit}"):
			return host.Result{ExitCode: 128, Stderr: "unknown revision\n"}, errors.New("git exited 128")
		default:
			return host.Result{}, fmt.Errorf("unexpected git call: %s", joined)
		}
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work/demo"}}}
	provider := NewProvider(runner, store)
	caps := newCaps(t, provider, &auditSink{})

	_, err := NewBroker(runner, store, caps).Push(context.Background(), PushSpec{Environment: "demo", Branch: "main", Force: true})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected missing tracking ref to fail closed, got err=%v calls=%v", err, runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "ls-remote") || strings.Contains(call, " push ") {
			t.Fatalf("network operation happened without a trusted local tracking baseline: %s", call)
		}
	}
}
