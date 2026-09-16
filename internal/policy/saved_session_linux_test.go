//go:build linux

package capability

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// An optional evaluator may support exact saved decisions without supporting
// a provider's broader scope. Admission must not silently downgrade that scope.
type exactSavedPolicy struct{ evaluator *FilePolicyEvaluator }

func (p exactSavedPolicy) Evaluate(ctx context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return p.evaluator.Evaluate(ctx, req)
}
func (p exactSavedPolicy) Remember(ctx context.Context, req core.CapabilityRequest, choice SavedChoice) error {
	return p.evaluator.Remember(ctx, req, choice)
}

func TestServicePersistsOnlyTrustedReusableScopeAndReportsSaveFailures(t *testing.T) {
	for _, scenario := range []string{"saved", "scope-unsupported", "save-failed", "recheck-failed"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			original := []byte(`{"default":"require-approval","rules":[]}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			evaluator := NewFilePolicyEvaluator(path)
			var policy PolicyEvaluator = evaluator
			if scenario == "scope-unsupported" {
				policy = exactSavedPolicy{evaluator}
			}
			provider := &scopeProvider{alter: func(request *core.CapabilityRequest) { request.Attributes["commit"] = "*" }}
			var events []core.CapabilityAuditEvent
			var savedFile []byte
			audit := configurationAuditFunc(func(_ context.Context, event core.CapabilityAuditEvent) error {
				events = append(events, event)
				if event.Type == "policy-saved" {
					var err error
					savedFile, err = os.ReadFile(path)
					if err != nil {
						return err
					}
					if scenario == "recheck-failed" {
						return os.WriteFile(path, []byte("{"), 0600)
					}
				}
				return nil
			})
			service := newTestService(t, policy, nil, audit, provider)
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111", Attributes: map[string]string{"branch": "main", "commit": "first"}, Parameters: map[string]string{"message": "opaque data"}}
			result, err := service.RequestWithDecision(context.Background(), request, func(_ context.Context, prompt core.ApprovalRequest) (ApprovalDecision, error) {
				if prompt.SavedScope.Attributes["commit"] != "*" || prompt.CapabilityRequest.Attributes["commit"] != "first" {
					t.Fatal("wrong proposed reusable scope", prompt)
				}
				if scenario == "save-failed" {
					if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
						return ApprovalDecision{}, err
					}
				}
				return ApprovalDecision{Approved: true, Save: AllowEnvironment}, nil
			})
			if scenario != "saved" {
				if err == nil || provider.calls != 0 || result.ExecutionState != core.CapabilityNotExecuted || result.AuditComplete {
					t.Fatal("failed persistence/recheck executed", result, err)
				}
				if scenario == "scope-unsupported" {
					data, readErr := os.ReadFile(path)
					if !errors.Is(err, core.ErrUnsupported) || readErr != nil || !bytes.Equal(data, original) {
						t.Fatal("unsupported scope changed Policy", err, readErr)
					}
				}
				if scenario == "save-failed" {
					last := events[len(events)-1]
					if last.Type != "policy-save-failed" || last.RequestID != result.RequestID || result.SavedChoice != "" || len(savedFile) != 0 {
						t.Fatal("failed save falsely acknowledged", result, last)
					}
					return
				}
				if scenario == "scope-unsupported" {
					return
				}
			} else if err != nil || provider.calls != 1 || result.Output != "opaque data" || !result.AuditComplete {
				t.Fatal("saved request failed", result, err)
			}
			if result.SavedChoice != string(AllowEnvironment) || len(savedFile) == 0 {
				t.Fatal("durable save lost from receipt", result)
			}
			persisted, err := decodePolicy(savedFile)
			if err != nil || len(persisted.SavedDecisions) != 1 {
				t.Fatal("saved file invalid", err)
			}
			rule := persisted.SavedDecisions[0]
			if rule.EnvironmentInstance != request.EnvironmentInstance || rule.Environment != "dev" || rule.Resource != "target" || rule.Attributes["branch"] != "main" || rule.Attributes["commit"] != "*" || bytes.Contains(savedFile, []byte("opaque data")) {
				t.Fatal("saved rule changed authority or included opaque data", rule)
			}
			if scenario == "recheck-failed" {
				return
			}
			request.Attributes["commit"] = "second"
			if result, err := service.Request(context.Background(), request); err != nil || result.ExecutionState != core.CapabilitySucceeded || provider.calls != 2 {
				t.Fatal("saved changing value not reusable", result, err)
			}
			request.Attributes["branch"] = "other"
			if result, err := service.Request(context.Background(), request); !errors.Is(err, core.ErrApprovalDenied) || result.ExecutionState != core.CapabilityNotExecuted || provider.calls != 2 {
				t.Fatal("saved scope widened fixed branch", result, err)
			}
		})
	}
}
