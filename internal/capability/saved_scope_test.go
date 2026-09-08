package capability

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type scopeProvider struct {
	fakeProvider
	alter func(*core.CapabilityRequest)
}

func (p *scopeProvider) SavedApprovalScope(_ context.Context, req core.CapabilityRequest) (core.CapabilityRequest, error) {
	p.alter(&req)
	return req, nil
}
func TestProviderScopeCannotReplaceAuthority(t *testing.T) {
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", Attributes: map[string]string{"branch": "main", "commit": "first"}}
	for _, mutate := range []func(*core.CapabilityRequest){
		func(r *core.CapabilityRequest) { r.Resource = "*" },
		func(r *core.CapabilityRequest) { delete(r.Attributes, "branch") },
		func(r *core.CapabilityRequest) { r.Environment = "other" },
		func(r *core.CapabilityRequest) { r.Action = "delete" },
		func(r *core.CapabilityRequest) { r.Attributes["branch"] = "other" },
		func(r *core.CapabilityRequest) { r.Attributes["new"] = "injected" },
		func(r *core.CapabilityRequest) { r.Parameters = map[string]string{"secret": "value"} },
	} {
		provider := &scopeProvider{alter: mutate}
		service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval}}, nil, &fakeAudit{}, provider)
		_, err := service.RequestWithDecision(context.Background(), request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
			t.Fatal("invalid scope reached approval")
			return ApprovalDecision{}, nil
		})
		if !errors.Is(err, core.ErrInvalidArgument) || provider.calls != 0 {
			t.Fatalf("invalid scope: %v", err)
		}
		if request.Attributes["branch"] != "main" {
			t.Fatal("provider mutated request")
		}
	}
}
func TestProviderScopeWildcardsOnlyDeclaredChangingAttributes(t *testing.T) {
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Attributes: map[string]string{"branch": "main", "commit": "first"}}
	provider := &scopeProvider{alter: func(r *core.CapabilityRequest) { r.Attributes["commit"] = "*" }}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval}}, nil, &fakeAudit{}, provider)
	_, err := service.RequestWithDecision(context.Background(), request, func(_ context.Context, p core.ApprovalRequest) (ApprovalDecision, error) {
		if p.CapabilityRequest.Attributes["commit"] != "first" || p.SavedScope.Attributes["commit"] != "*" || p.SavedScope.Attributes["branch"] != "main" {
			t.Fatal("scope and current operation confused")
		}
		return ApprovalDecision{Approved: true}, nil
	})
	if err != nil || provider.calls != 1 {
		t.Fatal(err)
	}
}

func TestObservedWildcardCannotBecomeSavedAuthority(t *testing.T) {
	req := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Attributes: map[string]string{"branch": "*"}}
	if _, err := savedApprovalScope(context.Background(), &fakeProvider{}, req); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("observed wildcard accepted")
	}
}
