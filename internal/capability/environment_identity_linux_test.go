//go:build linux

package capability

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSavedPolicySeparatesRecreatedEnvironmentName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	first, _ := core.NewEnvironmentInstanceID()
	second, _ := core.NewEnvironmentInstanceID()
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: first}
	if err := evaluator.Remember(context.Background(), request, AllowEnvironment); err != nil {
		t.Fatal(err)
	}
	eval, err := evaluator.Evaluate(context.Background(), request)
	if err != nil || eval.Decision != core.PolicyAllow {
		t.Fatal("same creation did not match")
	}
	request.EnvironmentInstance = second
	eval, err = evaluator.Evaluate(context.Background(), request)
	if err != nil || eval.Decision != core.PolicyRequireApproval {
		t.Fatal("recreated Environment inherited saved allow")
	}
	if err := evaluator.Remember(context.Background(), request, DenyEnvironment); err != nil {
		t.Fatal(err)
	}
	policy, err := evaluator.load()
	if err != nil || len(policy.SavedDecisions) != 2 {
		t.Fatal("new creation replaced old creation rule")
	}
	request.EnvironmentInstance = first
	eval, err = evaluator.Evaluate(context.Background(), request)
	if err != nil || eval.Decision != core.PolicyAllow {
		t.Fatal("old creation rule changed")
	}
	if err := evaluator.Remember(context.Background(), request, AllowGlobal); err != nil {
		t.Fatal(err)
	}
	request.EnvironmentInstance, _ = core.NewEnvironmentInstanceID()
	request.Environment = "other"
	eval, err = evaluator.Evaluate(context.Background(), request)
	if err != nil || eval.Decision != core.PolicyAllow {
		t.Fatal("explicit global scope did not cover another creation")
	}
}
func TestLegacySavedNameDoesNotMatchIdentifiedEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","saved_decisions":[{"capability":"local.echo","action":"echo","resource":"target","environment":"dev","decision":"allow"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	id, _ := core.NewEnvironmentInstanceID()
	evaluation, err := NewFilePolicyEvaluator(path).Evaluate(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: id})
	if err != nil || evaluation.Decision != core.PolicyRequireApproval {
		t.Fatal("legacy name-only grant adopted new identity")
	}
}

func TestAuditKeepsTrustedEnvironmentInstance(t *testing.T) {
	id, _ := core.NewEnvironmentInstanceID()
	audit := &fakeAudit{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, nil, audit, &fakeProvider{})
	_, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Environment: "dev", EnvironmentInstance: id})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range audit.events {
		if event.EnvironmentInstance != id {
			t.Fatal("audit lost Environment instance")
		}
	}
}
