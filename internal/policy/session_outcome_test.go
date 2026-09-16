package capability

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type outcomeSession struct {
	request             core.ApprovalRequest
	decision            ApprovalDecision
	beginErr, decideErr error
	nilSession          bool
	completed           int
	result              core.CapabilityResult
	err                 error
}

func (s *outcomeSession) Approve(context.Context, core.ApprovalRequest) (bool, error) {
	return false, errors.New("session provider fell back to a one-shot answer")
}
func (s *outcomeSession) BeginApproval(_ context.Context, request core.ApprovalRequest) (ApprovalSession, error) {
	s.request = request
	if s.beginErr != nil || s.nilSession {
		return nil, s.beginErr
	}
	return s, nil
}
func (s *outcomeSession) Decide(context.Context) (ApprovalDecision, error) {
	return s.decision, s.decideErr
}
func (s *outcomeSession) Complete(result core.CapabilityResult, err error) {
	s.completed++
	s.result, s.err = result, err
}

type executionIdentityProvider struct {
	fakeProvider
	requestID string
}

func (p *executionIdentityProvider) Execute(ctx context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	p.requestID = ExecutionRequestID(ctx)
	return p.fakeProvider.Execute(ctx, request)
}

func TestApprovalSessionReceivesActualExecutionAndAuditOutcome(t *testing.T) {
	for _, scenario := range []string{"allowed", "denied", "begin-failed", "nil-session", "answer-failed", "provider-failed", "audit-failed", "provider-and-audit-failed", "save-unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			failure := errors.New("fixture operation failed")
			approval := &outcomeSession{decision: ApprovalDecision{Approved: true}}
			provider := &executionIdentityProvider{}
			audit := &fakeAudit{}
			wantErr := error(nil)
			wantExecution := core.CapabilitySucceeded
			wantCompleted, wantCalls := 1, 1
			switch scenario {
			case "denied":
				approval.decision.Approved = false
				wantErr, wantExecution, wantCalls = core.ErrApprovalDenied, core.CapabilityNotExecuted, 0
			case "begin-failed":
				approval.beginErr = failure
				wantErr, wantExecution, wantCompleted, wantCalls = failure, core.CapabilityNotExecuted, 0, 0
			case "nil-session":
				approval.nilSession = true
				wantErr, wantExecution, wantCompleted, wantCalls = core.ErrIncompatibleState, core.CapabilityNotExecuted, 0, 0
			case "answer-failed":
				approval.decideErr = failure
				wantErr, wantExecution, wantCalls = failure, core.CapabilityNotExecuted, 0
			case "provider-failed":
				provider.err = failure
				wantErr, wantExecution = failure, core.CapabilityFailed
			case "audit-failed":
				audit.failAt = 4
				wantErr = core.ErrAuditIncomplete
			case "provider-and-audit-failed":
				provider.err, audit.failAt = failure, 4
				wantErr, wantExecution = failure, core.CapabilityFailed
			case "save-unsupported":
				approval.decision.Save = AllowGlobal
				wantErr, wantExecution, wantCalls = core.ErrUnsupported, core.CapabilityNotExecuted, 0
			}
			service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval}}, approval, audit, nil, provider)
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Attributes: map[string]string{"scope": "exact"}, Parameters: map[string]string{"message": "opaque result"}}
			result, err := service.Request(context.Background(), request)
			if !errors.Is(err, wantErr) || result.ExecutionState != wantExecution || provider.calls != wantCalls || approval.completed != wantCompleted {
				t.Fatalf("result=%+v error=%v executed=%d completed=%d", result, err, provider.calls, approval.completed)
			}
			if scenario == "provider-and-audit-failed" && !errors.Is(err, core.ErrAuditIncomplete) {
				t.Fatal("lost simultaneous audit failure", err)
			}
			if result.RequestID == "" || approval.request.RequestID != result.RequestID || len(approval.request.CapabilityRequest.Parameters) != 0 {
				t.Fatal("session received an unbound request or opaque parameters", approval.request)
			}
			if wantCompleted == 1 && (!reflect.DeepEqual(approval.result, result) || approval.err != err) {
				t.Fatal("session completion invented a different outcome", approval.result, approval.err)
			}
			if wantCalls == 1 {
				if provider.requestID != result.RequestID || result.Output != "opaque result" || result.AuditComplete != (audit.failAt == 0) {
					t.Fatal("execution identity or receipt lost", result, provider.requestID)
				}
			} else if result.Output != "" || result.AuditComplete || provider.requestID != "" {
				t.Fatal("unexecuted request claimed provider completion", result)
			}
			for _, event := range audit.events {
				if event.RequestID != result.RequestID {
					t.Fatal("audit and session identities diverged", event)
				}
			}
		})
	}
	if ExecutionRequestID(context.Background()) != "" {
		t.Fatal("unrelated context acquired execution identity")
	}
}

func TestMalformedCapabilityRequestsHaveNoApprovalExecutionOrAudit(t *testing.T) {
	for _, scenario := range []string{"no-capability", "no-action", "resource-control", "environment-control", "instance-without-environment", "invalid-instance", "empty-attribute", "attribute-control", "attribute-value-control", "empty-parameter", "padded-parameter", "parameter-control"} {
		t.Run(scenario, func(t *testing.T) {
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"}
			switch scenario {
			case "no-capability":
				request.Capability = ""
			case "no-action":
				request.Action = ""
			case "resource-control":
				request.Resource = "first\nsecond"
			case "environment-control":
				request.Environment = "dev\x00other"
			case "instance-without-environment":
				request.EnvironmentInstance = "env-11111111111111111111111111111111"
			case "invalid-instance":
				request.Environment, request.EnvironmentInstance = "dev", "invalid"
			case "empty-attribute":
				request.Attributes = map[string]string{"": "value"}
			case "attribute-control":
				request.Attributes = map[string]string{"a\nb": "value"}
			case "attribute-value-control":
				request.Attributes = map[string]string{"scope": "value\rnext"}
			case "empty-parameter":
				request.Parameters = map[string]string{"": "value"}
			case "padded-parameter":
				request.Parameters = map[string]string{" message ": "value"}
			case "parameter-control":
				request.Parameters = map[string]string{"a\x00b": "value"}
			}
			approval, provider, audit := &fakeApproval{approved: true}, &fakeProvider{}, &fakeAudit{}
			service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, approval, audit, provider)
			result, err := service.Request(context.Background(), request)
			if !errors.Is(err, core.ErrInvalidArgument) || !reflect.DeepEqual(result, core.CapabilityResult{}) || approval.calls != 0 || provider.calls != 0 || len(audit.events) != 0 {
				t.Fatal("malformed request crossed admission", result, err)
			}
		})
	}
}

func TestApprovalDisplaySnapshotIsPrivateAndIndependent(t *testing.T) {
	for _, withScope := range []bool{false, true} {
		request := core.ApprovalRequest{RequestID: "request", CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Attributes: map[string]string{"commit": "first"}, Parameters: map[string]string{"message": "private"}}}
		if withScope {
			scope := request.CapabilityRequest
			scope.Attributes = map[string]string{"commit": "*"}
			request.SavedScope = &scope
		}
		display := CloneApprovalRequest(request)
		if display.RequestID != "request" || display.CapabilityRequest.Parameters != nil || display.CapabilityRequest.Attributes["commit"] != "first" {
			t.Fatal("invalid display snapshot", display)
		}
		display.CapabilityRequest.Attributes["commit"] = "display-only"
		if request.CapabilityRequest.Attributes["commit"] != "first" || request.CapabilityRequest.Parameters["message"] != "private" {
			t.Fatal("display changed execution input")
		}
		if withScope {
			if display.SavedScope == request.SavedScope || display.SavedScope.Parameters != nil || display.SavedScope.Attributes["commit"] != "*" {
				t.Fatal("saved scope shared private state")
			}
			display.SavedScope.Attributes["commit"] = "display-only"
			if request.SavedScope.Attributes["commit"] != "*" || request.SavedScope.Parameters["message"] != "private" {
				t.Fatal("display changed saved authority")
			}
		} else if display.SavedScope != nil {
			t.Fatal("display invented a reusable scope")
		}
	}
}

func TestIncompleteOrUnknownCapabilityBoundaryNeverExecutes(t *testing.T) {
	for _, scenario := range []string{"policy-missing", "audit-missing", "provider-missing", "invalid-policy", "undeclared-parameter"} {
		t.Run(scenario, func(t *testing.T) {
			provider, audit := &fakeProvider{}, &fakeAudit{}
			var policy PolicyEvaluator = fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}
			var sink AuditSink = audit
			providers := []Provider{provider}
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo"}
			wantErr := core.ErrPolicyDenied
			switch scenario {
			case "policy-missing":
				policy = nil
			case "audit-missing":
				sink = nil
			case "provider-missing":
				providers, wantErr = nil, core.ErrUnsupported
			case "invalid-policy":
				policy = fakePolicy{evaluation: core.PolicyEvaluation{Decision: "unknown"}}
			case "undeclared-parameter":
				request.Parameters = map[string]string{"target": "unreviewed"}
			}
			result, err := newTestService(t, policy, nil, sink, providers...).Request(context.Background(), request)
			if !errors.Is(err, wantErr) || provider.calls != 0 || result.Output != "" || result.AuditComplete {
				t.Fatal("incomplete boundary executed", result, err)
			}
			if scenario == "invalid-policy" {
				if len(audit.events) != 2 || audit.events[1].Type != "policy-error" || audit.events[1].Reason != "invalid-policy-decision" || audit.events[1].Decision != core.PolicyDeny {
					t.Fatal("invalid Policy not audited as denial", audit.events)
				}
			} else if len(audit.events) != 0 {
				t.Fatal("rejected admission gained an audit identity", audit.events)
			}
		})
	}
}
