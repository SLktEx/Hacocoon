package egress

import (
	"context"
	"errors"
	"testing"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fixedPolicy struct {
	decision core.PolicyDecision
}

func (p fixedPolicy) Evaluate(_ context.Context, _ core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: p.decision, Reason: "test policy"}, nil
}

type fixedApproval struct {
	approved bool
	calls    int
}

func (a *fixedApproval) Approve(_ context.Context, _ core.ApprovalRequest) (bool, error) {
	a.calls++
	return a.approved, nil
}

type captureAudit struct {
	events []core.CapabilityAuditEvent
}

func (a *captureAudit) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func TestAuthorizeUsesExistingPolicyApprovalAndAuditBoundary(t *testing.T) {
	approval := &fixedApproval{approved: true}
	audit := &captureAudit{}
	service, err := capabilityapp.New(fixedPolicy{decision: core.PolicyRequireApproval}, approval, audit, Provider{})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := NewBroker(service).Authorize(context.Background(), core.EgressRequest{Environment: "env-a", Host: "example.com", Port: 443, Protocol: core.EgressHTTPS})
	if err != nil {
		t.Fatal(err)
	}
	if grant.RequestID == "" || approval.calls != 1 {
		t.Fatalf("grant=%#v approval.calls=%d", grant, approval.calls)
	}
	if len(audit.events) != 4 {
		t.Fatalf("audit events=%#v", audit.events)
	}
	if audit.events[0].Type != "requested" || audit.events[1].Type != "policy-decision" || audit.events[2].Type != "approval-decision" || audit.events[3].Type != "completed" {
		t.Fatalf("audit sequence=%#v", audit.events)
	}
	for _, event := range audit.events {
		if event.Environment != "env-a" || event.Resource != "example.com" || event.Attributes["protocol"] != "https" || event.Attributes["port"] != "443" {
			t.Fatalf("audit lost hostname authority scope: %#v", event)
		}
	}
}

func TestAuthorizeDenyDoesNotExecuteProvider(t *testing.T) {
	audit := &captureAudit{}
	service, err := capabilityapp.New(fixedPolicy{decision: core.PolicyDeny}, &fixedApproval{approved: true}, audit, Provider{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewBroker(service).Authorize(context.Background(), core.EgressRequest{Environment: "env-b", Host: "denied.example", Port: 443, Protocol: core.EgressHTTPS})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error=%v, want ErrPolicyDenied", err)
	}
	if len(audit.events) != 2 || audit.events[0].Type != "requested" || audit.events[1].Type != "policy-decision" {
		t.Fatalf("audit sequence=%#v", audit.events)
	}
}
