package capability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type PolicyEvaluator interface {
	Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error)
}

type ApprovalProvider interface {
	Approve(context.Context, core.ApprovalRequest) (bool, error)
}

type Provider interface {
	Capability() string
	Execute(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}

type AuditSink interface {
	Record(context.Context, core.CapabilityAuditEvent) error
}

type Service struct {
	policy    PolicyEvaluator
	approval  ApprovalProvider
	audit     AuditSink
	providers map[string]Provider
	now       func() time.Time
}

func New(policy PolicyEvaluator, approval ApprovalProvider, audit AuditSink, providers ...Provider) *Service {
	registered := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		if provider != nil {
			registered[provider.Capability()] = provider
		}
	}
	return &Service{policy: policy, approval: approval, audit: audit, providers: registered, now: time.Now}
}

func (s *Service) Request(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	req = normalizeRequest(req)
	if err := validateRequest(req); err != nil {
		return core.CapabilityResult{}, err
	}
	if s.policy == nil || s.audit == nil {
		return core.CapabilityResult{}, fmt.Errorf("capability boundary is incomplete: %w", core.ErrPolicyDenied)
	}
	if err := s.record(ctx, req, core.CapabilityAuditEvent{Type: "requested"}); err != nil {
		return core.CapabilityResult{}, fmt.Errorf("record capability request: %w", err)
	}

	evaluation, err := s.policy.Evaluate(ctx, req)
	if err != nil {
		_ = s.record(ctx, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: err.Error()})
		return core.CapabilityResult{}, fmt.Errorf("evaluate capability policy: %w", err)
	}
	if !validDecision(evaluation.Decision) {
		_ = s.record(ctx, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: "invalid policy decision"})
		return core.CapabilityResult{}, fmt.Errorf("invalid policy decision %q: %w", evaluation.Decision, core.ErrPolicyDenied)
	}
	if err := s.record(ctx, req, core.CapabilityAuditEvent{Type: "policy-decision", Decision: evaluation.Decision, Reason: evaluation.Reason}); err != nil {
		return core.CapabilityResult{}, fmt.Errorf("record policy decision: %w", err)
	}

	switch evaluation.Decision {
	case core.PolicyDeny:
		return core.CapabilityResult{}, fmt.Errorf("%s: %w", evaluation.Reason, core.ErrPolicyDenied)
	case core.PolicyRequireApproval:
		if s.approval == nil {
			return core.CapabilityResult{}, fmt.Errorf("approval provider unavailable: %w", core.ErrApprovalDenied)
		}
		approved, approvalErr := s.approval.Approve(ctx, core.ApprovalRequest{CapabilityRequest: req, Reason: evaluation.Reason})
		if approvalErr != nil {
			falseValue := false
			_ = s.record(ctx, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &falseValue, Reason: approvalErr.Error()})
			return core.CapabilityResult{}, fmt.Errorf("obtain capability approval: %w", approvalErr)
		}
		if err := s.record(ctx, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &approved, Reason: evaluation.Reason}); err != nil {
			return core.CapabilityResult{}, fmt.Errorf("record approval decision: %w", err)
		}
		if !approved {
			return core.CapabilityResult{}, core.ErrApprovalDenied
		}
	}

	provider, ok := s.providers[req.Capability]
	if !ok {
		return core.CapabilityResult{}, fmt.Errorf("capability provider %q: %w", req.Capability, core.ErrUnsupported)
	}
	result, execErr := provider.Execute(ctx, req)
	success := execErr == nil
	completionErr := s.record(ctx, req, core.CapabilityAuditEvent{Type: "completed", Success: &success, Reason: errorText(execErr)})
	if execErr != nil && completionErr != nil {
		return result, errors.Join(execErr, fmt.Errorf("record capability completion: %w", completionErr))
	}
	if execErr != nil {
		return result, execErr
	}
	if completionErr != nil {
		return result, fmt.Errorf("capability completed but audit failed: %w", completionErr)
	}
	return result, nil
}

func (s *Service) record(ctx context.Context, req core.CapabilityRequest, event core.CapabilityAuditEvent) error {
	event.Time = s.now().UTC()
	event.Capability = req.Capability
	event.Action = req.Action
	event.Resource = req.Resource
	event.Environment = req.Environment
	event.Attributes = cloneStrings(req.Attributes)
	return s.audit.Record(ctx, event)
}

func normalizeRequest(req core.CapabilityRequest) core.CapabilityRequest {
	req.Capability = strings.TrimSpace(req.Capability)
	req.Action = strings.TrimSpace(req.Action)
	req.Resource = strings.TrimSpace(req.Resource)
	req.Environment = strings.TrimSpace(req.Environment)
	if req.Attributes != nil {
		normalized := make(map[string]string, len(req.Attributes))
		for key, value := range req.Attributes {
			normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		req.Attributes = normalized
	}
	return req
}

func validateRequest(req core.CapabilityRequest) error {
	if req.Capability == "" || req.Action == "" {
		return core.ErrInvalidArgument
	}
	for _, value := range []string{req.Capability, req.Action, req.Resource, req.Environment} {
		if strings.ContainsAny(value, "\r\n\x00") {
			return core.ErrInvalidArgument
		}
	}
	for key, value := range req.Attributes {
		if key == "" || strings.ContainsAny(key, "\r\n\x00") || strings.ContainsAny(value, "\r\n\x00") {
			return core.ErrInvalidArgument
		}
	}
	return nil
}

func validDecision(decision core.PolicyDecision) bool {
	return decision == core.PolicyAllow || decision == core.PolicyDeny || decision == core.PolicyRequireApproval
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func cloneStrings(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
