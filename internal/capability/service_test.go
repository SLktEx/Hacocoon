package capability

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

type fakeProvider struct {
	calls int
	err   error
}

func (*fakeProvider) Capability() string { return "local.echo" }
func (f *fakeProvider) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	f.calls++
	return core.CapabilityResult{Provider: "local.echo", Output: req.Parameters["message"]}, f.err
}

func TestServiceAllowExecutesAndAuditsWithoutParameters(t *testing.T) {
	audit := &fakeAudit{}
	provider := &fakeProvider{}
	service := New(fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, provider)
	service.now = func() time.Time { return time.Unix(1, 0) }
	result, err := service.Request(context.Background(), core.CapabilityRequest{
		Capability: "local.echo", Action: "echo", Resource: "safe", Parameters: map[string]string{"message": "secret-value"},
	})
	if err != nil || result.Output != "secret-value" || provider.calls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, provider.calls, err)
	}
	if len(audit.events) != 3 || audit.events[0].Type != "requested" || audit.events[1].Decision != core.PolicyAllow || audit.events[2].Type != "completed" {
		t.Fatalf("events=%#v", audit.events)
	}
	for _, event := range audit.events {
		if strings.Contains(fmt.Sprintf("%#v", event), "secret-value") {
			t.Fatalf("secret parameter leaked into audit event: %#v", event)
		}
	}
}

func TestServiceDenyNeverExecutes(t *testing.T) {
	provider := &fakeProvider{}
	audit := &fakeAudit{}
	service := New(fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyDeny, Reason: "blocked"}}, &fakeApproval{}, audit, provider)
	_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if !errors.Is(err, core.ErrPolicyDenied) || provider.calls != 0 {
		t.Fatalf("calls=%d err=%v", provider.calls, err)
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
			service := New(fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval, Reason: "sensitive"}}, approval, &fakeAudit{}, provider)
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
			service := New(tc.policy, &fakeApproval{approved: true}, tc.audit, provider)
			_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
			if err == nil || provider.calls != 0 {
				t.Fatalf("calls=%d err=%v", provider.calls, err)
			}
		})
	}
}

func TestServiceSurfacesPostExecutionAuditFailure(t *testing.T) {
	provider := &fakeProvider{}
	service := New(fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, &fakeAudit{failAt: 3}, provider)
	result, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Parameters: map[string]string{"message": "done"}})
	if result.Output != "done" || err == nil || provider.calls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, provider.calls, err)
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

func TestStdioApprovalDefaultsDeny(t *testing.T) {
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
		approved, err := NewStdioApproval(strings.NewReader(tc.input), &out).Approve(context.Background(), core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "sensitive"}})
		if err != nil || approved != tc.want || !strings.Contains(out.String(), "[y/N]") {
			t.Fatalf("input=%q approved=%t prompt=%q err=%v", tc.input, approved, out.String(), err)
		}
	}
}
