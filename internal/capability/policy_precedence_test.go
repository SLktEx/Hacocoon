package capability

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyRestrictionsWinRegardlessOfRuleOrder(t *testing.T) {
	decisions := []core.PolicyDecision{core.PolicyAllow, core.PolicyRequireApproval, core.PolicyDeny}
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		policy := PolicyFile{Default: core.PolicyAllow}
		for _, index := range order {
			policy.Rules = append(policy.Rules, PolicyRule{Capability: "local.echo", Action: "echo", Resource: "*", Environment: "*", Decision: decisions[index]})
		}
		path := filepath.Join(t.TempDir(), "policy.json")
		save := func() {
			data, err := json.Marshal(policy)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		save()
		request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}
		got, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), request)
		if err != nil || got.Decision != core.PolicyDeny {
			t.Fatalf("order %v bypassed deny: %#v %v", order, got, err)
		}
		for index := range policy.Rules {
			if policy.Rules[index].Decision == core.PolicyDeny {
				policy.Rules[index].Environment = "other"
			}
		}
		save()
		got, err = NewFilePolicyEvaluator(path).Evaluate(context.Background(), request)
		if err != nil || got.Decision != core.PolicyRequireApproval {
			t.Fatalf("order %v bypassed approval or widened deny: %#v %v", order, got, err)
		}
	}
}
func TestExplicitAllowStillOverridesDefaultDeny(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	data := []byte(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"target","environment":"dev","decision":"allow"}]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	for _, env := range []string{"dev", "other"} {
		got, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: env})
		want := core.PolicyDeny
		if env == "dev" {
			want = core.PolicyAllow
		}
		if err != nil || got.Decision != want {
			t.Fatalf("fallback changed: %#v %v", got, err)
		}
	}
}
