package capability

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ApprovalDecision keeps a one-shot decision separate from optional persistence.
type ApprovalDecision struct {
	Approved bool
	Save     SavedChoice
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
