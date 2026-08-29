package capability

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestProviderFailureAuditUsesStableNonSecretReason(t *testing.T) {
	const secret = "ghp_SUPERSECRET_SHOULD_NOT_BE_AUDITED"
	audit := &fakeAudit{}
	provider := &fakeProvider{err: errors.New("transport failed: Authorization: Bearer " + secret + " /home/operator/.ssh/id_ed25519")}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, provider)

	result, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if err == nil || !strings.Contains(err.Error(), secret) {
		t.Fatalf("immediate caller should retain provider diagnostic: %v", err)
	}
	if result.ExecutionState != core.CapabilityFailed || !result.AuditComplete {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(audit.events) != 3 {
		t.Fatalf("events=%#v", audit.events)
	}
	completed := audit.events[2]
	if completed.Type != "completed" || completed.Reason != "provider-execution-failed" {
		t.Fatalf("completed event=%#v", completed)
	}
	if strings.Contains(completed.Reason, secret) || strings.Contains(completed.Reason, "/home/operator") {
		t.Fatalf("provider diagnostic leaked into audit: %#v", completed)
	}
}

func TestProviderFailureAuditPreservesOnlyKnownErrorCategory(t *testing.T) {
	const secret = "token=TOP_SECRET"
	audit := &fakeAudit{}
	provider := &fakeProvider{err: errors.Join(errors.New(secret), core.ErrCapabilityStale)}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, provider)

	_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"})
	if !errors.Is(err, core.ErrCapabilityStale) || !strings.Contains(err.Error(), secret) {
		t.Fatalf("returned error lost caller diagnostics/category: %v", err)
	}
	completed := audit.events[len(audit.events)-1]
	if completed.Reason != "capability-stale" || strings.Contains(completed.Reason, secret) {
		t.Fatalf("completed event=%#v", completed)
	}
}

func TestPolicyAndApprovalProviderErrorsAreNotPersistedVerbatim(t *testing.T) {
	const secret = "credential=DO_NOT_LOG"

	policyAudit := &fakeAudit{}
	policyService := newTestService(t, fakePolicy{err: errors.New(secret)}, &fakeApproval{}, policyAudit, &fakeProvider{})
	if _, err := policyService.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"}); err == nil {
		t.Fatal("expected policy error")
	}
	if got := policyAudit.events[len(policyAudit.events)-1].Reason; got != "policy-evaluation-failed" || strings.Contains(got, secret) {
		t.Fatalf("policy audit reason=%q", got)
	}

	approvalAudit := &fakeAudit{}
	approval := &fakeApproval{err: errors.New(secret)}
	approvalService := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval, Reason: "review"}}, approval, approvalAudit, &fakeProvider{})
	if _, err := approvalService.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"}); err == nil {
		t.Fatal("expected approval error")
	}
	if got := approvalAudit.events[len(approvalAudit.events)-1].Reason; got != "approval-provider-failed" || strings.Contains(got, secret) {
		t.Fatalf("approval audit reason=%q", got)
	}
}
