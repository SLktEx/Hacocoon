package capability

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type changingPolicy struct {
	next    core.PolicyDecision
	failure error
	calls   int
}

func (p *changingPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	p.calls++
	if p.calls == 1 {
		return core.PolicyEvaluation{Decision: core.PolicyAllow}, nil
	}
	return core.PolicyEvaluation{Decision: p.next}, p.failure
}
func TestPolicyRecheckRefusesRevokedInitialAllow(t *testing.T) {
	for _, decision := range []core.PolicyDecision{core.PolicyDeny, core.PolicyRequireApproval, "invalid"} {
		provider := &fakeProvider{}
		service := newTestService(t, &changingPolicy{next: decision}, nil, &fakeAudit{}, provider)
		_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"})
		if err == nil || provider.calls != 0 {
			t.Fatalf("executed after Policy changed to %s", decision)
		}
	}
	provider := &fakeProvider{}
	service := newTestService(t, &changingPolicy{failure: errors.New("invalid policy update")}, nil, &fakeAudit{}, provider)
	if _, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"}); err == nil || provider.calls != 0 {
		t.Fatal("ignored policy reload failure")
	}
}
func TestOneShotApprovalCannotOverridePolicyEditedWhileWaiting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{}
	service := newTestService(t, NewFilePolicyEvaluator(path), nil, &fakeAudit{}, provider)
	_, err := service.RequestWithDecision(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"}, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
		if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
			return ApprovalDecision{}, err
		}
		return ApprovalDecision{Approved: true}, nil
	})
	if !errors.Is(err, core.ErrPolicyDenied) || provider.calls != 0 {
		t.Fatalf("stale approval executed: %v", err)
	}
}
