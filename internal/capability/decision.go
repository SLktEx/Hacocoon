package capability

import (
	"context"
	"maps"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ApprovalDecision keeps a one-shot decision separate from optional persistence.
type ApprovalDecision struct {
	Approved bool
	Save     SavedChoice
}

// ApprovalSession binds waiting and completion to one admitted request. Complete
// receives the actual application outcome and must not block on a client.
type ApprovalSession interface {
	Decide(context.Context) (ApprovalDecision, error)
	Complete(core.CapabilityResult, error)
}

type SessionApprovalProvider interface {
	BeginApproval(context.Context, core.ApprovalRequest) (ApprovalSession, error)
}

// CloneApprovalRequest creates a display snapshot without opaque parameters.
func CloneApprovalRequest(request core.ApprovalRequest) core.ApprovalRequest {
	request.CapabilityRequest.Attributes = maps.Clone(request.CapabilityRequest.Attributes)
	request.CapabilityRequest.Parameters = nil
	if request.SavedScope != nil {
		scope := *request.SavedScope
		scope.Attributes = maps.Clone(scope.Attributes)
		scope.Parameters = nil
		request.SavedScope = &scope
	}
	return request
}

// ValidateApprovalDecision rejects a saved choice inconsistent with the current
// answer or trusted scope. It does not save Policy or grant execution authority.
func ValidateApprovalDecision(request core.ApprovalRequest, decision ApprovalDecision) error {
	if decision.Save == "" {
		return nil
	}
	rule, err := approvalSavedRule(request, decision.Save)
	if err != nil {
		return err
	}
	if rule.Decision != core.PolicyRequireApproval && (rule.Decision == core.PolicyAllow) != decision.Approved {
		return core.ErrInvalidArgument
	}
	return nil
}

type decisionFunc func(context.Context, core.ApprovalRequest) (ApprovalDecision, error)

func (f decisionFunc) Decide(ctx context.Context, r core.ApprovalRequest) (ApprovalDecision, error) {
	return f(ctx, r)
}
func (f decisionFunc) Approve(ctx context.Context, r core.ApprovalRequest) (bool, error) {
	d, err := f(ctx, r)
	return d.Approved, err
}

func (s *Service) RequestWithDecision(ctx context.Context, r core.CapabilityRequest, decide func(context.Context, core.ApprovalRequest) (ApprovalDecision, error)) (core.CapabilityResult, error) {
	var provider ApprovalProvider
	if decide != nil {
		provider = decisionFunc(decide)
	}
	return s.request(ctx, r, provider)
}
