package capability

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func packageBaselineRule() PolicyRule {
	return PolicyRule{
		Capability:  "network.egress",
		Action:      "connect",
		Resource:    "archive.ubuntu.com",
		Environment: "*",
		Attributes:  map[string]string{"protocol": "https", "port": "443"},
		Decision:    core.PolicyAllow,
		Reason:      "official Ubuntu package repository",
	}
}

func TestBaselineAllowAppliesBeforeDefaultButAfterExplicitRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	evaluator := NewFilePolicyEvaluator(path)
	if err := evaluator.ConfigureBaselineRules([]PolicyRule{packageBaselineRule()}); err != nil {
		t.Fatal(err)
	}
	request := core.CapabilityRequest{
		Capability:  "network.egress",
		Action:      "connect",
		Resource:    "archive.ubuntu.com",
		Environment: "dev",
		Attributes:  map[string]string{"protocol": "https", "port": "443"},
	}

	got, err := evaluator.Evaluate(context.Background(), request)
	if err != nil || got.Decision != core.PolicyAllow || got.Reason != "official Ubuntu package repository" {
		t.Fatalf("baseline grant missing: %#v %v", got, err)
	}

	for _, tc := range []struct {
		name     string
		decision core.PolicyDecision
	}{
		{name: "approval", decision: core.PolicyRequireApproval},
		{name: "deny", decision: core.PolicyDeny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`{"default":"deny","rules":[{"capability":"network.egress","action":"connect","resource":"archive.ubuntu.com","environment":"dev","attributes":{"protocol":"https","port":"443"},"decision":"` + string(tc.decision) + `","reason":"operator restriction"}]}`)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := evaluator.Evaluate(context.Background(), request)
			if err != nil || got.Decision != tc.decision || got.Reason != "operator restriction" {
				t.Fatalf("baseline bypassed explicit %s: %#v %v", tc.decision, got, err)
			}
		})
	}
}

func TestBaselineAllowDoesNotWidenHostPortOrProtocol(t *testing.T) {
	evaluator := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
	if err := evaluator.ConfigureBaselineRules([]PolicyRule{packageBaselineRule()}); err != nil {
		t.Fatal(err)
	}
	base := core.CapabilityRequest{
		Capability:  "network.egress",
		Action:      "connect",
		Resource:    "archive.ubuntu.com",
		Environment: "dev",
		Attributes:  map[string]string{"protocol": "https", "port": "443"},
	}
	for _, mutate := range []func(*core.CapabilityRequest){
		func(r *core.CapabilityRequest) { r.Resource = "packages.example.com" },
		func(r *core.CapabilityRequest) { r.Attributes = map[string]string{"protocol": "https", "port": "444"} },
		func(r *core.CapabilityRequest) { r.Attributes = map[string]string{"protocol": "http", "port": "443"} },
	} {
		request := base
		request.Attributes = map[string]string{"protocol": "https", "port": "443"}
		mutate(&request)
		got, err := evaluator.Evaluate(context.Background(), request)
		if err != nil || got.Decision != core.PolicyDeny {
			t.Fatalf("baseline widened authority for %#v: %#v %v", request, got, err)
		}
	}
}

func TestBaselineRulesRejectBroadOrMutableAuthority(t *testing.T) {
	for _, rule := range []PolicyRule{
		{Capability: "network.egress", Action: "connect", Resource: "*", Environment: "*", Decision: core.PolicyAllow, Reason: "broad"},
		{Capability: "network.egress", Action: "connect", Resource: "archive.ubuntu.com", Environment: "*", Attributes: map[string]string{"protocol": "*", "port": "443"}, Decision: core.PolicyAllow, Reason: "wildcard"},
		{Capability: "network.egress", Action: "connect", Resource: "archive.ubuntu.com", Environment: "dev", Decision: core.PolicyAllow, Reason: "scoped"},
		{Capability: "network.egress", Action: "connect", Resource: "archive.ubuntu.com", Environment: "*", Decision: core.PolicyDeny, Reason: "deny"},
	} {
		evaluator := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
		if err := evaluator.ConfigureBaselineRules([]PolicyRule{rule}); err == nil {
			t.Fatalf("accepted unsafe baseline rule: %#v", rule)
		}
	}
}

func TestBaselineRulesCloneCallerAttributesAndConfigureOnce(t *testing.T) {
	rule := packageBaselineRule()
	evaluator := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
	if err := evaluator.ConfigureBaselineRules([]PolicyRule{rule}); err != nil {
		t.Fatal(err)
	}
	rule.Attributes["port"] = "444"
	request := core.CapabilityRequest{
		Capability: "network.egress", Action: "connect", Resource: "archive.ubuntu.com", Environment: "dev",
		Attributes: map[string]string{"protocol": "https", "port": "443"},
	}
	got, err := evaluator.Evaluate(context.Background(), request)
	if err != nil || got.Decision != core.PolicyAllow {
		t.Fatalf("caller mutation changed configured baseline: %#v %v", got, err)
	}
	if err := evaluator.ConfigureBaselineRules(nil); err == nil {
		t.Fatal("baseline evaluator allowed reconfiguration")
	}
}
