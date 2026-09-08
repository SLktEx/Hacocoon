//go:build linux

package capability

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// These exercise the common Policy/approval/audit boundary with real capability
// identities and target shapes. They do not execute Git or network providers.
func TestGitAndNetworkShareSavedPolicyAuditAndRecreationSemantics(t *testing.T) {
	requests := []core.CapabilityRequest{
		{Capability: "git.repository", Action: "push", Resource: "https://github.com/SLktEx/Hacocoon-test.git", Attributes: map[string]string{"repository": "repo-example", "target_ref": "refs/heads/main", "update_kind": "fast-forward"}},
		{Capability: "network.egress", Action: "connect", Resource: "example.com", Attributes: map[string]string{"protocol": "https", "port": "443"}},
	}
	choices := []SavedChoice{AllowEnvironment, DenyEnvironment, AskEnvironment, AllowGlobal, DenyGlobal, AskGlobal}
	for _, request := range requests {
		for _, choice := range choices {
			t.Run(request.Capability+"/"+string(choice), func(t *testing.T) {
				ctx := context.Background()
				request.Environment = "dev"
				request.EnvironmentInstance = "env-11111111111111111111111111111111"
				path := filepath.Join(t.TempDir(), "policy.json")
				if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
				policy := NewFilePolicyEvaluator(path)
				audit := &fakeAudit{}
				service := newTestService(t, policy, nil, audit, namedProvider(request.Capability))
				approved := choice == AllowEnvironment || choice == AllowGlobal
				result, err := service.RequestWithDecision(ctx, request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
					return ApprovalDecision{Approved: approved, Save: choice}, nil
				})
				wantError := core.ErrApprovalDenied
				if approved {
					wantError = nil
				} else if choice == DenyEnvironment || choice == DenyGlobal {
					wantError = core.ErrPolicyDenied
				}
				if !errors.Is(err, wantError) {
					t.Fatalf("current decision: %v", err)
				}
				if result.SavedChoice != string(choice) {
					t.Fatal("saved receipt differs")
				}
				file, err := policy.load()
				if err != nil || len(file.SavedDecisions) != 1 {
					t.Fatalf("saved policy: %+v %v", file, err)
				}
				rule := file.SavedDecisions[0]
				if rule.Capability != request.Capability || rule.Action != request.Action || rule.Resource != request.Resource || !maps.Equal(rule.Attributes, request.Attributes) {
					t.Fatal("saved target broadened")
				}
				global := choice == AllowGlobal || choice == DenyGlobal || choice == AskGlobal
				if global {
					if rule.Environment != "*" || rule.EnvironmentInstance != "" {
						t.Fatal("global scope differs")
					}
				} else if rule.Environment != "dev" || rule.EnvironmentInstance != request.EnvironmentInstance {
					t.Fatal("Environment identity missing")
				}
				found := false
				for _, event := range audit.events {
					if event.Type == "policy-saved" {
						found = true
						if event.SavedChoice != string(choice) || event.EnvironmentInstance != request.EnvironmentInstance || event.SavedScope == nil || event.SavedScope.Resource != rule.Resource || event.SavedScope.Environment != rule.Environment || event.SavedScope.EnvironmentInstance != rule.EnvironmentInstance || !maps.Equal(event.SavedScope.Attributes, rule.Attributes) {
							t.Fatal("audit scope differs from saved policy")
						}
					}
				}
				if !found {
					t.Fatal("missing saved-policy audit")
				}
				evaluation, err := policy.Evaluate(ctx, request)
				if err != nil || evaluation.Decision != rule.Decision {
					t.Fatalf("reevaluation: %+v %v", evaluation, err)
				}
				recreated := request
				recreated.EnvironmentInstance = "env-22222222222222222222222222222222"
				evaluation, err = policy.Evaluate(ctx, recreated)
				want := core.PolicyRequireApproval
				if global {
					want = rule.Decision
				}
				if err != nil || evaluation.Decision != want {
					t.Fatalf("recreated identity: %+v %v", evaluation, err)
				}
				changed := request
				changed.Resource = "different-target.example"
				evaluation, err = policy.Evaluate(ctx, changed)
				if err != nil || evaluation.Decision != core.PolicyRequireApproval {
					t.Fatal("saved choice covered another target")
				}
				changed = request
				changed.Attributes = maps.Clone(request.Attributes)
				if request.Capability == "git.repository" {
					changed.Attributes["target_ref"] = "refs/heads/other"
				} else {
					changed.Attributes["port"] = "8443"
				}
				evaluation, err = policy.Evaluate(ctx, changed)
				if err != nil || evaluation.Decision != core.PolicyRequireApproval {
					t.Fatal("saved choice covered another ref or port")
				}
				if choice == AskEnvironment || choice == AskGlobal {
					prompted := false
					_, err = service.RequestWithDecision(ctx, request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
						prompted = true
						return ApprovalDecision{}, nil
					})
					if !prompted || !errors.Is(err, core.ErrApprovalDenied) {
						t.Fatal("saved ask authorized a later operation")
					}
				}
			})
		}
	}
}
