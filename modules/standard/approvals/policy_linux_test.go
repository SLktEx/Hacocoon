//go:build linux

package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestQueuedSavedChoicesUseActualPolicyAndExecutionBoundary(t *testing.T) {
	for _, choice := range []capability.SavedChoice{capability.AllowEnvironment, capability.DenyEnvironment, capability.AskEnvironment, capability.AllowGlobal, capability.DenyGlobal, capability.AskGlobal} {
		t.Run(string(choice), func(t *testing.T) {
			q := New()
			p := &echoProvider{}
			path := filepath.Join(t.TempDir(), "policy.json")
			const original = `{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"example.com","environment":"dev","attributes":{"port":"443"},"decision":"require-approval"}]}`
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			policy := capability.NewFilePolicyEvaluator(path)
			s, err := capability.New(policy, q, auditSink{}, p)
			if err != nil {
				t.Fatal(err)
			}
			req := request()
			req.Environment = "dev"
			req.EnvironmentInstance, err = core.NewEnvironmentInstanceID()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan completion, 1)
			go func() { r, err := s.Request(ctx, req); done <- completion{r, err} }()
			prompt := pending(t, q)
			approved := choice == capability.AllowEnvironment || choice == capability.AllowGlobal
			result, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: approved, Save: choice})
			if err != nil || result.SavedChoice != string(choice) {
				t.Fatalf("saved receipt: %+v %v", result, err)
			}
			executed := receive(t, done)
			if approved && (executed.err != nil || p.calls.Load() != 1) {
				t.Fatalf("allow: %+v", executed)
			}
			if !approved && (executed.err == nil || p.calls.Load() != 0) {
				t.Fatalf("deny/ask answer: %+v", executed)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var saved capability.PolicyFile
			if err := json.Unmarshal(contents, &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved.SavedDecisions) != 1 || len(saved.Rules) != 1 || saved.Default != core.PolicyDeny {
				t.Fatal("saved choice replaced administrator Policy")
			}
			// Remove only the administrator ask fixture to observe the persisted choice.
			saved.Rules = nil
			data, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			evaluation, err := policy.Evaluate(ctx, req)
			if err != nil || evaluation.Decision != saved.SavedDecisions[0].Decision {
				t.Fatalf("saved Policy is not reusable: %+v %v", evaluation, err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("saved Policy is not private")
			}
		})
	}
}

func TestPolicyChangeWhileQueuedStillPreventsExecution(t *testing.T) {
	q := New()
	p := &echoProvider{}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := capability.New(capability.NewFilePolicyEvaluator(path), q, auditSink{}, p)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	if err := os.WriteFile(path, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true})
	if !errors.Is(err, core.ErrPolicyDenied) || result.ExecutionState != core.CapabilityNotExecuted || p.calls.Load() != 0 {
		t.Fatalf("changed Policy bypassed: %+v %v", result, err)
	}
	if r := receive(t, done); !errors.Is(r.err, core.ErrPolicyDenied) {
		t.Fatal("request lost policy refusal")
	}
}

func TestSavedPolicyAuditFailureIsNotAcknowledgedOrExecuted(t *testing.T) {
	q := New()
	p := &echoProvider{}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := capability.New(capability.NewFilePolicyEvaluator(path), q, auditSink{failType: "policy-saved"}, p)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	result, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true, Save: capability.AllowGlobal})
	if !errors.Is(err, core.ErrAuditIncomplete) || result.SavedChoice != "" || p.calls.Load() != 0 {
		t.Fatalf("unverified save acknowledged: %+v %v", result, err)
	}
	if r := receive(t, done); !errors.Is(r.err, core.ErrAuditIncomplete) {
		t.Fatal("request lost audit failure")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var policy capability.PolicyFile
	if err := json.Unmarshal(data, &policy); err != nil || len(policy.SavedDecisions) != 1 {
		t.Fatal("test did not reach durable persistence")
	}
}
