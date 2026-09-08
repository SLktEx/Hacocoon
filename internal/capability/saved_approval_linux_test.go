//go:build linux

package capability

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedGlobalScopeAuditMatchesPersistedAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	audit := &fakeAudit{}
	service := newTestService(t, NewFilePolicyEvaluator(path), nil, audit, &fakeProvider{})
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}
	result, err := service.RequestWithDecision(context.Background(), request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
		return ApprovalDecision{Approved: true, Save: AllowGlobal}, nil
	})
	if err != nil || result.SavedChoice != string(AllowGlobal) {
		t.Fatalf("global save: %+v %v", result, err)
	}
	found := false
	for _, event := range audit.events {
		if event.Type == "policy-saved" {
			found = true
			if event.Environment != "dev" || event.EnvironmentInstance != request.EnvironmentInstance || event.SavedScope == nil || event.SavedScope.Environment != "*" || event.SavedScope.EnvironmentInstance != "" {
				t.Fatal("audit confused current Environment with global saved scope")
			}
		}
	}
	if !found {
		t.Fatal("missing saved scope audit")
	}
}

func TestSavedApprovalAuditsBeforeSavingAndPreservesPromptAuthority(t *testing.T) {
	for _, failAt := range []int{0, 3, 4} {
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		evaluator := NewFilePolicyEvaluator(path)
		audit := &fakeAudit{failAt: failAt}
		provider := &fakeProvider{}
		service := newTestService(t, evaluator, nil, audit, provider)
		request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111", Attributes: map[string]string{"branch": "main"}, Parameters: map[string]string{"message": "opaque"}}
		result, err := service.RequestWithDecision(context.Background(), request, func(_ context.Context, prompt core.ApprovalRequest) (ApprovalDecision, error) {
			if len(prompt.CapabilityRequest.Parameters) != 0 {
				t.Fatal("opaque parameters sent to approval UI")
			}
			prompt.CapabilityRequest.Attributes["branch"] = "other"
			prompt.SavedScope.Attributes["branch"] = "injected"
			return ApprovalDecision{Approved: true, Save: AllowEnvironment}, nil
		})
		policy, loadErr := evaluator.load()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if failAt == 0 {
			if err != nil || result.SavedChoice != string(AllowEnvironment) || provider.calls != 1 || result.Output != "opaque" || len(policy.SavedDecisions) != 1 || policy.SavedDecisions[0].Attributes["branch"] != "main" {
				t.Fatalf("saved flow failed: %#v %v", result, err)
			}
		} else {
			if err == nil || result.SavedChoice != "" || provider.calls != 0 {
				t.Fatal("executed after failed audit")
			}
			if failAt == 3 && len(policy.SavedDecisions) != 0 {
				t.Fatal("saved before approval audit")
			}
			if failAt == 4 && !errors.Is(err, core.ErrAuditIncomplete) {
				t.Fatal("post-save audit failure not distinguished")
			}
		}
	}
}

func TestSavedAskIsDurableAndNeverBecomesAllow(t *testing.T) {
	for _, choice := range []SavedChoice{AskEnvironment, AskGlobal} {
		for _, approved := range []bool{false, true} {
			path := filepath.Join(t.TempDir(), "policy.json")
			if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			evaluator := NewFilePolicyEvaluator(path)
			provider := &fakeProvider{}
			service := newTestService(t, evaluator, nil, &fakeAudit{}, provider)
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}
			_, err := service.RequestWithDecision(context.Background(), request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
				return ApprovalDecision{Approved: approved, Save: choice}, nil
			})
			if approved && err != nil {
				t.Fatal(err)
			}
			if !approved && !errors.Is(err, core.ErrApprovalDenied) {
				t.Fatalf("declined ask: %v", err)
			}
			wantCalls := 0
			if approved {
				wantCalls = 1
			}
			if provider.calls != wantCalls {
				t.Fatal("saving ask changed one-shot execution")
			}
			policy, err := evaluator.load()
			if err != nil {
				t.Fatal(err)
			}
			if len(policy.SavedDecisions) != 1 || policy.SavedDecisions[0].Decision != core.PolicyRequireApproval {
				t.Fatal("ask was not persisted")
			}
			scope := "dev"
			if choice == AskGlobal {
				scope = "*"
			}
			if policy.SavedDecisions[0].Environment != scope {
				t.Fatal("wrong ask scope")
			}
			prompted := false
			_, err = service.RequestWithDecision(context.Background(), request, func(context.Context, core.ApprovalRequest) (ApprovalDecision, error) {
				prompted = true
				return ApprovalDecision{}, nil
			})
			if !prompted || !errors.Is(err, core.ErrApprovalDenied) || provider.calls != wantCalls {
				t.Fatal("saved ask auto-authorized next request")
			}
		}
	}
}
