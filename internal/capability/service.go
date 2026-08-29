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

// NonAuthorityParameterProvider explicitly declares opaque parameters that do
// not change the authority, scope, target, credential, or security meaning of
// an operation. All authority-sensitive inputs belong in Attributes.
type NonAuthorityParameterProvider interface {
	NonAuthorityParameters() []string
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
	provider, ok := s.providers[req.Capability]
	if !ok {
		return core.CapabilityResult{}, fmt.Errorf("capability provider %q: %w", req.Capability, core.ErrUnsupported)
	}
	if err := validateNonAuthorityParameters(provider, req.Parameters); err != nil {
		return core.CapabilityResult{}, err
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
		_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: "policy-evaluation-failed"})
		return baseResult, fmt.Errorf("evaluate capability policy: %w", err)
	}
	if !validDecision(evaluation.Decision) {
		_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "policy-error", Decision: core.PolicyDeny, Reason: "invalid-policy-decision"})
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
			_ = s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &falseValue, Reason: "approval-provider-failed"})
			return baseResult, fmt.Errorf("obtain capability approval: %w", approvalErr)
		}
		if err := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "approval-decision", Approved: &approved, Reason: evaluation.Reason}); err != nil {
			return baseResult, fmt.Errorf("record approval decision: %w", err)
		}
		if !approved {
			return baseResult, core.ErrApprovalDenied
		}
	}

	result, execErr := provider.Execute(ctx, req)
	result.RequestID = requestID
	if execErr == nil {
		result.ExecutionState = core.CapabilitySucceeded
	} else {
		result.ExecutionState = core.CapabilityFailed
	}
	success := execErr == nil
	completionErr := s.record(ctx, requestID, req, core.CapabilityAuditEvent{Type: "completed", Success: &success, Reason: auditErrorReason(execErr)})
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
	for key := range req.Parameters {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n\x00") {
			return core.ErrInvalidArgument
		}
	}
	return nil
}

func validateNonAuthorityParameters(provider Provider, parameters map[string]string) error {
	if len(parameters) == 0 {
		return nil
	}
	declarer, ok := provider.(NonAuthorityParameterProvider)
	if !ok {
		return fmt.Errorf("capability %q does not permit opaque parameters: %w", provider.Capability(), core.ErrPolicyDenied)
	}
	allowed := make(map[string]struct{})
	for _, key := range declarer.NonAuthorityParameters() {
		key = strings.TrimSpace(key)
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	for key := range parameters {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("parameter %q for capability %q is not declared non-authority: %w", key, provider.Capability(), core.ErrPolicyDenied)
		}
	}
	return nil
}

func validDecision(decision core.PolicyDecision) bool {
	return decision == core.PolicyAllow || decision == core.PolicyDeny || decision == core.PolicyRequireApproval
}

func randomRequestID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// auditErrorReason deliberately converts provider failures into a closed set of
// stable categories. Provider error strings can contain untrusted subprocess
// stderr, credentials, local paths, or other diagnostics that must not become
// durable security-audit data. The original error is still returned to the
// immediate caller by Request.
func auditErrorReason(err error) string {
	if err == nil {
		return ""
	}
	for _, candidate := range []struct {
		err    error
		reason string
	}{
		{context.Canceled, "context-canceled"},
		{context.DeadlineExceeded, "context-deadline-exceeded"},
		{core.ErrCapabilityStale, "capability-stale"},
		{core.ErrInvalidArgument, "invalid-argument"},
		{core.ErrNotFound, "not-found"},
		{core.ErrAlreadyExists, "already-exists"},
		{core.ErrUnsupported, "unsupported"},
		{core.ErrUnsafeShrink, "unsafe-shrink"},
		{core.ErrRuntimeUnavailable, "runtime-unavailable"},
		{core.ErrStorageUnavailable, "storage-unavailable"},
		{core.ErrStorageBusy, "storage-busy"},
		{core.ErrWorkspaceBusy, "workspace-busy"},
		{core.ErrPolicyDenied, "policy-denied"},
		{core.ErrApprovalDenied, "approval-denied"},
		{core.ErrAuditIncomplete, "audit-incomplete"},
		{core.ErrIncompatibleState, "incompatible-state"},
		{core.ErrRecoveryRequired, "recovery-required"},
	} {
		if errors.Is(err, candidate.err) {
			return candidate.reason
		}
	}
	return "provider-execution-failed"
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
