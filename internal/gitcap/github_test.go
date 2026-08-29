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

const (
	shaA = "1111111111111111111111111111111111111111"
	shaB = "2222222222222222222222222222222222222222"
)

type envStore struct{ env core.Environment }

func (s envStore) GetEnvironment(context.Context, string) (core.Environment, error) { return s.env, nil }

type allowPolicy struct{}

func (allowPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyAllow}, nil
}

type noApproval struct{}

func (noApproval) Approve(context.Context, core.ApprovalRequest) (bool, error) {
	return false, errors.New("unexpected approval")
}

type auditSink struct{ events []core.CapabilityAuditEvent }

func (a *auditSink) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

type runnerFunc struct {
	fn    func(name string, args []string) (host.Result, error)
	calls []string
}

func (r *runnerFunc) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.fn == nil { return host.Result{}, nil }
	return r.fn(name, args)
}

func newCaps(t *testing.T, provider capabilityapp.Provider, audit capabilityapp.AuditSink) *capabilityapp.Service {
	t.Helper()
	caps, err := capabilityapp.New(allowPolicy{}, noApproval{}, audit, provider)
	if err != nil { t.Fatal(err) }
	return caps
}

func TestParseGitHubRemote(t *testing.T) {
	for _, raw := range []string{"https://github.com/acme/demo.git", "https://github.com/acme/demo", "git@github.com:acme/demo.git", "ssh://git@github.com/acme/demo.git"} {
		repo, err := ParseGitHubRemote(raw)
		if err != nil || repo.Owner != "acme" || repo.Name != "demo" { t.Fatalf("remote=%q repo=%#v err=%v", raw, repo, err) }
	}
	for _, raw := range []string{"https://token@github.com/acme/demo.git", "ssh://root@github.com/acme/demo.git", "https://gitlab.com/acme/demo.git", "file:///tmp/demo.git", "https://github.com/acme/demo/extra.git", "https://github.com/acme/demo.git?token=secret"} {
		if _, err := ParseGitHubRemote(raw); !errors.Is(err, core.ErrInvalidArgument) { t.Fatalf("remote=%q err=%v", raw, err) }
	}
}

func TestBrokerPushUsesApprovedExactSourceSHA(t *testing.T) {
	workspace := "/work/demo"
	runner := &runnerFunc{fn: func(_ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "check-ref-format --branch feature/x"): return host.Result{}, nil
		case strings.Contains(joined, "config --get remote.origin.url"): return host.Result{Stdout: "https://github.com/acme/demo.git\n"}, nil
		case strings.Contains(joined, "rev-parse --verify HEAD^{commit}"): return host.Result{Stdout: shaA + "\n"}, nil
		case strings.Contains(joined, "cat-file -e "+shaA+"^{commit}"): return host.Result{}, nil
		case strings.Contains(joined, "push --porcelain origin "+shaA+":refs/heads/feature/x"): return host.Result{Stdout: "ok\n"}, nil
		default: return host.Result{}, fmt.Errorf("unexpected git call: %s", joined)
		}
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: workspace}}}
	provider := NewProvider(runner, store)
	audit := &auditSink{}
	broker := NewBroker(runner, store, newCaps(t, provider, audit))
	result, err := broker.Push(context.Background(), PushSpec{Environment: "demo", Branch: "feature/x"})
	if err != nil || result.Provider != GitHubCapability || result.RequestID == "" || result.ExecutionState != core.CapabilitySucceeded { t.Fatalf("result=%#v err=%v calls=%v", result, err, runner.calls) }
	last := runner.calls[len(runner.calls)-1]
	if !strings.Contains(last, shaA+":refs/heads/feature/x") || strings.Contains(last, "HEAD:refs/heads/feature/x") { t.Fatalf("push was not pinned to approved SHA: %s", last) }
	decision := audit.events[1]
	if decision.Resource != "github://acme/demo/refs/heads/feature/x" || decision.Attributes["source_sha"] != shaA || decision.Attributes["organization"] != "acme" || decision.Attributes["repository"] != "demo" || decision.RequestID == "" { t.Fatalf("audit decision=%#v", decision) }
}

func TestProviderRejectsRemoteMutationAfterPolicy(t *testing.T) {
	calls := 0
	runner := &runnerFunc{fn: func(_ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "config --get remote.origin.url") {
			calls++
			if calls == 1 { return host.Result{Stdout: "https://github.com/acme/demo.git\n"}, nil }
			return host.Result{Stdout: "https://github.com/acme/other.git\n"}, nil
		}
		if strings.Contains(joined, "check-ref-format") { return host.Result{}, nil }
		if strings.Contains(joined, "rev-parse") { return host.Result{Stdout: shaA + "\n"}, nil }
		return host.Result{}, nil
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work/demo"}}}
	provider := NewProvider(runner, store)
	_, err := NewBroker(runner, store, newCaps(t, provider, &auditSink{})).Push(context.Background(), PushSpec{Environment: "demo", Branch: "feature/x"})
	if !errors.Is(err, core.ErrCapabilityStale) { t.Fatalf("err=%v calls=%v", err, runner.calls) }
	for _, call := range runner.calls { if strings.Contains(call, " push ") { t.Fatalf("push executed after remote mutation: %s", call) } }
}

func TestForcePushRejectsRemoteRefMutation(t *testing.T) {
	lsCalls := 0
	runner := &runnerFunc{fn: func(_ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "check-ref-format"): return host.Result{}, nil
		case strings.Contains(joined, "config --get remote.origin.url"): return host.Result{Stdout: "https://github.com/acme/demo.git\n"}, nil
		case strings.Contains(joined, "rev-parse"): return host.Result{Stdout: shaB + "\n"}, nil
		case strings.Contains(joined, "cat-file -e"): return host.Result{}, nil
		case strings.Contains(joined, "ls-remote --refs origin refs/heads/main"):
			lsCalls++; sha := shaA; if lsCalls > 1 { sha = shaB }; return host.Result{Stdout: sha + "\trefs/heads/main\n"}, nil
		default: return host.Result{}, fmt.Errorf("unexpected call %s", joined)
		}
	}}
	store := envStore{env: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work/demo"}}}
	provider := NewProvider(runner, store)
	_, err := NewBroker(runner, store, newCaps(t, provider, &auditSink{})).Push(context.Background(), PushSpec{Environment: "demo", Branch: "main", Force: true})
	if !errors.Is(err, core.ErrCapabilityStale) { t.Fatalf("err=%v calls=%v", err, runner.calls) }
	for _, call := range runner.calls { if strings.Contains(call, " push ") { t.Fatalf("force push executed after target mutation: %s", call) } }
}
