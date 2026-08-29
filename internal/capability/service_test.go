package capability

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakePolicy struct {
	evaluation core.PolicyEvaluation
	err        error
}

func (f fakePolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return f.evaluation, f.err
}

type fakeApproval struct {
	approved bool
	err      error
	calls    int
}

func (f *fakeApproval) Approve(context.Context, core.ApprovalRequest) (bool, error) {
	f.calls++
	return f.approved, f.err
}

type fakeAudit struct {
	events []core.CapabilityAuditEvent
	failAt int
}

func (f *fakeAudit) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	f.events = append(f.events, event)
	if f.failAt > 0 && len(f.events) == f.failAt {
		return errors.New("audit unavailable")
	}
	return nil
}

type synchronizedAudit struct {
	mu     sync.Mutex
	events []core.CapabilityAuditEvent
}

func (a *synchronizedAudit) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
	return nil
}

type fakeProvider struct {
	calls int
	err   error
}

func (*fakeProvider) Capability() string { return "local.echo" }
func (f *fakeProvider) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	f.calls++
	return core.CapabilityResult{Provider: "local.echo", Output: req.Parameters["message"]}, f.err
}

type statelessProvider struct{}

func (statelessProvider) Capability() string { return "local.echo" }
func (statelessProvider) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	return core.CapabilityResult{Provider: "local.echo", Output: req.Resource}, nil
}

type namedProvider string

func (p namedProvider) Capability() string { return string(p) }
func (p namedProvider) Execute(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
	return core.CapabilityResult{Provider: string(p)}, nil
}

func newTestService(t *testing.T, policy PolicyEvaluator, approval ApprovalProvider, audit AuditSink, providers ...Provider) *Service {
	t.Helper()
	service, err := New(policy, approval, audit, providers...)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestServiceAllowExecutesAndAuditsWithoutParameters(t *testing.T) {
	audit := &fakeAudit{}
	provider := &fakeProvider{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, provider)
	service.now = func() time.Time { return time.Unix(1, 0) }
	result, err := service.Request(context.Background(), core.CapabilityRequest{
		Capability: "local.echo", Action: "echo", Resource: "safe", Parameters: map[string]string{"message": "secret-value"},
	})
	if err != nil || result.Output != "secret-value" || provider.calls != 1 || result.ExecutionState != core.CapabilitySucceeded || !result.AuditComplete {
		t.Fatalf("result=%#v calls=%d err=%v", result, provider.calls, err)
	}
	if result.RequestID == "" || len(audit.events) != 3 || audit.events[0].Type != "requested" || audit.events[1].Decision != core.PolicyAllow || audit.events[2].Type != "completed" {
		t.Fatalf("result=%#v events=%#v", result, audit.events)
	}
	for _, event := range audit.events {
		if event.RequestID != result.RequestID {
			t.Fatalf("event request id=%q result=%q", event.RequestID, result.RequestID)
		}
		if strings.Contains(fmt.Sprintf("%#v", event), "secret-value") {
			t.Fatalf("secret parameter leaked into audit event: %#v", event)
		}
	}
}

func TestServiceDenyNeverExecutes(t *testing.T) {
	provider := &fakeProvider{}
	audit := &fakeAudit{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyDeny, Reason: "blocked"}}, &fakeApproval{}, audit, provider)
	result, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if !errors.Is(err, core.ErrPolicyDenied) || provider.calls != 0 || result.ExecutionState != core.CapabilityNotExecuted || result.RequestID == "" {
		t.Fatalf("result=%#v calls=%d err=%v", result, provider.calls, err)
	}
}

func TestServiceRequiresExplicitApproval(t *testing.T) {
	for _, tc := range []struct {
		name     string
		approved bool
		wantErr  error
		calls    int
	}{
		{name: "approved", approved: true, calls: 1},
		{name: "denied", approved: false, wantErr: core.ErrApprovalDenied, calls: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeProvider{}
			approval := &fakeApproval{approved: tc.approved}
			service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval, Reason: "sensitive"}}, approval, &fakeAudit{}, provider)
			_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
			if !errors.Is(err, tc.wantErr) || provider.calls != tc.calls || approval.calls != 1 {
				t.Fatalf("provider calls=%d approval=%d err=%v", provider.calls, approval.calls, err)
			}
		})
	}
}

func TestServiceFailsClosedBeforeProviderOnPolicyOrAuditFailure(t *testing.T) {
	cases := []struct {
		name   string
		policy PolicyEvaluator
		audit  *fakeAudit
	}{
		{name: "policy", policy: fakePolicy{err: errors.New("bad policy")}, audit: &fakeAudit{}},
		{name: "request audit", policy: fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, audit: &fakeAudit{failAt: 1}},
		{name: "decision audit", policy: fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, audit: &fakeAudit{failAt: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeProvider{}
			service := newTestService(t, tc.policy, &fakeApproval{approved: true}, tc.audit, provider)
			_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
			if err == nil || provider.calls != 0 {
				t.Fatalf("calls=%d err=%v", provider.calls, err)
			}
		})
	}
}

func TestServiceDistinguishesPostExecutionAuditFailure(t *testing.T) {
	provider := &fakeProvider{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, &fakeAudit{failAt: 3}, provider)
	result, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Parameters: map[string]string{"message": "done"}})
	if result.Output != "done" || !errors.Is(err, core.ErrAuditIncomplete) || provider.calls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, provider.calls, err)
	}
	if result.ExecutionState != core.CapabilitySucceeded || result.AuditComplete || result.RequestID == "" {
		t.Fatalf("ambiguous result=%#v", result)
	}
}

func TestFilePolicyEvaluatorMatchesSpecificRuleAndDefaultsDeny(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "policy.json")
	policy := `{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"safe","decision":"allow"},{"capability":"local.echo","action":"echo","resource":"sensitive","decision":"require-approval","reason":"human check"}]}`
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	for _, tc := range []struct {
		resource string
		want     core.PolicyDecision
	}{
		{resource: "safe", want: core.PolicyAllow},
		{resource: "sensitive", want: core.PolicyRequireApproval},
		{resource: "other", want: core.PolicyDeny},
	} {
		got, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: tc.resource})
		if err != nil || got.Decision != tc.want {
			t.Fatalf("resource=%s got=%#v err=%v", tc.resource, got, err)
		}
	}
}

func TestFilePolicyRejectsUnknownFieldsAndImplicitWildcard(t *testing.T) {
	for _, policy := range []string{
		`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resouce":"safe","decision":"allow"}]}`,
		`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","decision":"allow"}]}`,
	} {
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "unsafe"}); err == nil {
			t.Fatalf("policy must fail closed: %s", policy)
		}
	}

	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"*","decision":"allow"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "anything"})
	if err != nil || got.Decision != core.PolicyAllow {
		t.Fatalf("explicit wildcard got=%#v err=%v", got, err)
	}
}

func TestFilePolicyMakesEnvironmentAndParametersVisible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	policy := `{"default":"deny","rules":[` +
		`{"capability":"git","action":"push","resource":"org/repo","environment":"trusted","parameters":{"branch":"main"},"decision":"allow"},` +
		`{"capability":"git","action":"push","resource":"org/repo","environment":"sandbox","parameters":{"branch":"main"},"decision":"require-approval"}` +
		`]}`
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	cases := []struct {
		environment string
		branch      string
		want        core.PolicyDecision
	}{
		{environment: "trusted", branch: "main", want: core.PolicyAllow},
		{environment: "sandbox", branch: "main", want: core.PolicyRequireApproval},
		{environment: "trusted", branch: "dev", want: core.PolicyDeny},
	}
	for _, tc := range cases {
		got, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{
			Capability: "git", Action: "push", Resource: "org/repo", Environment: tc.environment, Parameters: map[string]string{"branch": tc.branch},
		})
		if err != nil || got.Decision != tc.want {
			t.Fatalf("env=%s branch=%s got=%#v err=%v", tc.environment, tc.branch, got, err)
		}
	}
}

func TestMissingPolicyDefaultsDenyAndMalformedPolicyFails(t *testing.T) {
	evaluator := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "missing.json"))
	got, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if err != nil || got.Decision != core.PolicyDeny {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), core.CapabilityRequest{}); err == nil {
		t.Fatal("malformed policy must fail closed")
	}
}

func TestStdioApprovalShowsSecurityContextAndDefaultsDeny(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{input: "y\n", want: true},
		{input: "YES\n", want: true},
		{input: "\n", want: false},
		{input: "no\n", want: false},
	} {
		var out bytes.Buffer
		approved, err := NewStdioApproval(strings.NewReader(tc.input), &out).Approve(context.Background(), core.ApprovalRequest{
			CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "sensitive", Environment: "demo", Parameters: map[string]string{"branch": "main"}},
			Reason:            "human check",
		})
		prompt := out.String()
		if err != nil || approved != tc.want || !strings.Contains(prompt, "[y/N]") || !strings.Contains(prompt, "environment=demo") || !strings.Contains(prompt, `branch="main"`) || !strings.Contains(prompt, "reason=human check") {
			t.Fatalf("input=%q approved=%t prompt=%q err=%v", tc.input, approved, prompt, err)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("display broken") }

func TestApprovalPromptFailureNeverExecutesProvider(t *testing.T) {
	provider := &fakeProvider{}
	service := newTestService(t,
		fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval, Reason: "sensitive"}},
		NewStdioApproval(strings.NewReader("yes\n"), failingWriter{}),
		&fakeAudit{},
		provider,
	)
	_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if err == nil || provider.calls != 0 {
		t.Fatalf("provider calls=%d err=%v", provider.calls, err)
	}
}

func TestConcurrentRequestsHaveDistinctCorrelatedIDs(t *testing.T) {
	audit := &synchronizedAudit{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, statelessProvider{})

	var wg sync.WaitGroup
	for _, resource := range []string{"one", "two"} {
		resource := resource
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: resource})
			if err != nil || result.RequestID == "" {
				t.Errorf("resource=%s result=%#v err=%v", resource, result, err)
			}
		}()
	}
	wg.Wait()

	audit.mu.Lock()
	defer audit.mu.Unlock()
	groups := map[string]int{}
	for _, event := range audit.events {
		if event.RequestID == "" {
			t.Fatal("audit event missing request id")
		}
		groups[event.RequestID]++
	}
	if len(groups) != 2 {
		t.Fatalf("request groups=%#v events=%#v", groups, audit.events)
	}
	for id, count := range groups {
		if count != 3 {
			t.Fatalf("request %s has %d events, want 3", id, count)
		}
	}
}

func TestNewRejectsDuplicateAndInvalidProviderNames(t *testing.T) {
	if _, err := New(fakePolicy{}, &fakeApproval{}, &fakeAudit{}, &fakeProvider{}, &fakeProvider{}); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("duplicate provider error=%v", err)
	}
	if _, err := New(fakePolicy{}, &fakeApproval{}, &fakeAudit{}, namedProvider("")); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("empty provider error=%v", err)
	}
}
