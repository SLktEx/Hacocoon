//go:build linux

package aws

import (
	"context"
	"encoding/json"
	"errors"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestAWSFourPoliciesPersistAndKeepAccountActionResourceScope(t *testing.T) {
	for _, choice := range []capabilityapp.SavedChoice{capabilityapp.AllowEnvironment, capabilityapp.AllowGlobal, capabilityapp.DenyEnvironment, capabilityapp.DenyGlobal, capabilityapp.AskEnvironment, capabilityapp.AskGlobal} {
		t.Run(string(choice), func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "policy.json")
			if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			h := fixtureHost(t, &calls)
			policy := capabilityapp.NewFilePolicyEvaluator(path)
			svc, err := capabilityapp.New(policy, nil, capabilityapp.NewJSONLAudit(filepath.Join(t.TempDir(), "events.jsonl")), &Provider{Host: h})
			if err != nil {
				t.Fatal(err)
			}
			svc.ConfigureEnvironmentIdentity(environments{})
			var prepared core.CapabilityRequest
			b := Broker{Host: h, Environments: environments{}, Capabilities: requester(func(_ context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
				prepared = r
				return core.CapabilityResult{}, nil
			})}
			if _, err := b.List(ctx, ListSpec{Environment: "dev", URL: "s3://example-bucket/project/"}); err != nil {
				t.Fatal(err)
			}
			approved := choice != capabilityapp.DenyEnvironment && choice != capabilityapp.DenyGlobal
			_, err = svc.RequestWithDecision(ctx, prepared, func(_ context.Context, prompt core.ApprovalRequest) (capabilityapp.ApprovalDecision, error) {
				if prompt.SavedScope == nil || prompt.SavedScope.Attributes["account"] != testIdentity.Account || prompt.SavedScope.Attributes["principal"] != testIdentity.Principal {
					t.Fatal("wrong review details")
				}
				return capabilityapp.ApprovalDecision{Approved: approved, Save: choice}, nil
			})
			if approved && err != nil || !approved && !errors.Is(err, core.ErrApprovalDenied) && !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatal(err)
			}
			// Reopen the same manual configuration; no provider-specific allow database.
			reopened := capabilityapp.NewFilePolicyEvaluator(path)
			r := prepared
			r.EnvironmentInstance, _ = environments{}.EnvironmentInstance(ctx, core.Environment{Name: "dev"})
			got, err := reopened.Evaluate(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			want := core.PolicyAllow
			if !approved {
				want = core.PolicyDeny
			}
			if choice == capabilityapp.AskEnvironment || choice == capabilityapp.AskGlobal {
				want = core.PolicyRequireApproval
			}
			if got.Decision != want {
				t.Fatal(got)
			}
			encoded, _ := json.Marshal(r)
			var changed core.CapabilityRequest
			json.Unmarshal(encoded, &changed)
			for field, value := range map[string]string{"account": "999999999999", "principal": "arn:aws:iam::123456789012:user/Other", "region": "us-east-1", "profile": "other", "prefix": "other/", "bucket_owner": "999999999999"} {
				json.Unmarshal(encoded, &changed)
				changed.Attributes[field] = value
				if decision, _ := reopened.Evaluate(ctx, changed); decision.Decision != core.PolicyRequireApproval {
					t.Fatalf("saved scope crossed %s", field)
				}
			}
			r.Environment = "other"
			r.EnvironmentInstance, _ = environments{}.EnvironmentInstance(ctx, core.Environment{Name: "other"})
			decision, _ := reopened.Evaluate(ctx, r)
			if choice == capabilityapp.AllowGlobal || choice == capabilityapp.DenyGlobal {
				if decision.Decision != want {
					t.Fatal("lost global scope")
				}
			} else if decision.Decision != core.PolicyRequireApproval {
				t.Fatal("crossed Environment")
			}
			// Manual revocation applies without restarting the service.
			if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			before := calls
			if _, err := svc.Request(ctx, prepared); !errors.Is(err, core.ErrPolicyDenied) || calls != before {
				t.Fatal("revoked rule executed", err)
			}
		})
	}
}
