package capability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	policy       PolicyEvaluator
	approval     ApprovalProvider
	audit        AuditSink
	providers    map[string]Provider
	now          func() time.Time
	newRequestID func() (string, error)
}

func New(policy PolicyEvaluator, approval ApprovalProvider, audit AuditSink, providers ...Provider) (*Service, error) {
	registered := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		name := provider.Capability()
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, "\r\n\x00") {
			return nil, fmt.Errorf("invalid capability provider name %q: %w", name, core.ErrInvalidArgument)
		}
		if _, exists := registered[name]; exists {
			return nil, fmt.Errorf("duplicate capability provider %q: %w", name, core.ErrAlreadyExists)
		}
		registered[name] = provider
	}
	return &Service{
		policy:       policy,
		approval:     approval,
		audit:        audit,
		providers:    registered,
		now:          time.Now,
		newRequestID: randomRequestID,
	}, nil
}

func (s *Service) Request(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	req = normalizeRequest(req)
	if err := validateRequest(req); err != nil {
		return core.CapabilityResult{}, err
	}
	if s.policy == nil || s.audit == nil {
		return core.CapabilityResult{}, fmt.Errorf("capability boundary is incomplete: %w", core.ErrPolicyDenied)
	}
	requestID, err := s.newRequestID()
	if err != nil {
		return core.CapabilityResult{}, fmt.Errorf("create capability request id: %w", err)
	}
	baseResult := core.CapabilityResult{RequestID: requestID, ExecutionState: core.CapabilityNotExecuted}
	if err := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "requested"}); err != nil {
		return baseResult, fmt.Errorf("record capability request: %w", err)
	}

	evaluation, err := s.policy.Evaluate(ctx, req)
	if err != nil {
		_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: err.Error()})
		return baseResult, fmt.Errorf("evaluate capability policy: %w", err)
	}
	if !validDecision(evaluation.Decision) {
		_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: "invalid policy decision"})
		return baseResult, fmt.Errorf("invalid policy decision %q: %w", evaluation.Decision, core.ErrPolicyDenied)
	}
	if err := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "policy-decision", Decision: evaluation.Decision, Reason: evaluation.Reason}); err != nil {
		return baseResult, fmt.Errorf("record policy decision: %w", err)
	}

	switch evaluation.Decision {
	case core.PolicyDeny:
		return baseResult, fmt.Errorf("%s: %w", evaluation.Reason, core.ErrPolicyDenied)
	case core.PolicyRequireApproval:
		if s.approval == nil {
			return baseResult, fmt.Errorf("approval provider unavailable: %w", core.ErrApprovalDenied)
		}
		approved, approvalErr := s.approval.Approve(ctx, core.ApprovalRequest{CapabilityRequest: req, Reason: evaluation.Reason})
		if approvalErr != nil {
			falseValue := false
			_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &falseValue, Reason: approvalErr.Error()})
			return baseResult, fmt.Errorf("obtain capability approval: %w", approvalErr)
		}
		if err := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &approved, Reason: evaluation.Reason}); err != nil {
			return baseResult, fmt.Errorf("record approval decision: %w", err)
		}
		if !approved {
			return baseResult, core.ErrApprovalDenied
		}
	}

	provider, ok := s.providers[req.Capability]
	if !ok {
		return baseResult, fmt.Errorf("capability provider %q: %w", req.Capability, core.ErrUnsupported)
	}
	result, execErr := provider.Execute(ctx, req)
	result.RequestID = requestID
	if execErr == nil {
		result.ExecutionState = core.CapabilitySucceeded
	} else {
		result.ExecutionState = core.CapabilityFailed
	}
	success := execErr == nil
	completionErr := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "completed", Success: &success, Reason: errorText(execErr)})
	result.AuditComplete = completionErr == nil
	if execErr != nil && completionErr != nil {
		return result, errors.Join(execErr, core.ErrAuditIncomplete, fmt.Errorf("record capability completion: %w", completionErr))
	}
	if execErr != nil {
		return result, execErr
	}
	if completionErr != nil {
		return result, errors.Join(core.ErrAuditIncomplete, fmt.Errorf("record capability completion: %w", completionErr))
	}
	return result, nil
}

func (s *Service) record(ctx context.Context, requestID string, req core.CapabilityRequest, event core.CapabilityAuditEvent) error {
	event.Time = s.now().UTC()
	event.RequestID = requestID
	event.Capability = req.Capability
	event.Action = req.Action
	event.Resource = req.Resource
	event.Environment = req.Environment
	return s.audit.Record(ctx, event)
}

func normalizeRequest(req core.CapabilityRequest) core.CapabilityRequest {
	req.Capability = strings.TrimSpace(req.Capability)
	req.Action = strings.TrimSpace(req.Action)
	req.Resource = strings.TrimSpace(req.Resource)
	req.Environment = strings.TrimSpace(req.Environment)
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
	for key, value := range req.Parameters {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key+value, "\r\n\x00") {
			return core.ErrInvalidArgument
		}
	}
	return nil
}

func validDecision(decision core.PolicyDecision) bool {
	return decision == core.PolicyAllow || decision == core.PolicyDeny || decision == core.PolicyRequireApproval
}

func randomRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
