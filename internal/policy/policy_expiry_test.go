package capability

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPolicyExpiryBoundaryAndRestrictionPrecedence(t *testing.T) {
	deadline := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, rules string
		now         time.Time
		want        core.PolicyDecision
	}{
		{"before", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":"2026-09-12T12:00:00Z"}]`, deadline.Add(-time.Nanosecond), core.PolicyAllow},
		{"at", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":"2026-09-12T12:00:00Z"}]`, deadline, core.PolicyDeny},
		{"after", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":"2026-09-12T21:00:00+09:00"}]`, deadline.Add(time.Second), core.PolicyDeny},
		{"unbounded", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow"}]`, deadline, core.PolicyAllow},
		{"deny-wins", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":"2026-09-13T12:00:00Z"},{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"deny"}]`, deadline, core.PolicyDeny},
		{"deny-expired", `[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow"},{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"deny","expires_at":"2026-09-12T12:00:00Z"}]`, deadline, core.PolicyAllow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			if err := os.WriteFile(path, []byte(`{"default":"deny","rules":`+tc.rules+`}`), 0600); err != nil {
				t.Fatal(err)
			}
			evaluator := NewFilePolicyEvaluator(path)
			evaluator.now = func() time.Time { return tc.now }
			result, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"})
			if err != nil || result.Decision != tc.want {
				t.Fatalf("decision=%s err=%v; want %s", result.Decision, err, tc.want)
			}
		})
	}
}

func TestApprovalCannotExecuteAfterItsPolicyRuleExpires(t *testing.T) {
	deadline := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"require-approval","expires_at":"2026-09-12T12:00:00Z"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	now := deadline.Add(-time.Second)
	evaluator.now = func() time.Time { return now }
	provider := &fakeProvider{}
	service := newTestService(t, evaluator, nil, &fakeAudit{}, provider)
	_, err := service.RequestWithDecision(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"}, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
		now = deadline
		return ApprovalDecision{Approved: true}, nil
	})
	if !errors.Is(err, core.ErrPolicyDenied) || provider.calls != 0 {
		t.Fatalf("expired approval executed: calls=%d err=%v", provider.calls, err)
	}
}

func TestPolicyRejectsMalformedExpiryAndExpiresSavedGlobalRule(t *testing.T) {
	for _, value := range []string{`"tomorrow"`, `"2026-09-12"`, `123`, `{}`} {
		_, err := decodePolicy([]byte(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":` + value + `}]}`))
		if err == nil {
			t.Fatalf("accepted malformed expiry %s", value)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[],"saved_decisions":[{"capability":"local.echo","action":"echo","resource":"target","environment":"*","decision":"allow","expires_at":"2000-01-01T00:00:00Z"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "new-env"})
	if err != nil || result.Decision != core.PolicyDeny {
		t.Fatalf("expired saved rule was reused: %+v %v", result, err)
	}
}
