//go:build linux

package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func persistenceRequest() core.CapabilityRequest {
	return core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}
}

func TestPolicyWritersRefuseUnsafeLockAndParentWithoutChangingAuthority(t *testing.T) {
	for _, kind := range []string{"lock-symlink", "lock-hardlink", "lock-public", "lock-directory", "parent-symlink", "parent-public", "parent-missing"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.json")
			original := []byte(`{"default":"deny","rules":[]}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(t.TempDir(), "unrelated")
			if err := os.WriteFile(victim, []byte("preserve unrelated file"), 0600); err != nil {
				t.Fatal(err)
			}
			lock := filepath.Join(dir, ".policy-save.lock")
			writerPath := path
			var err error
			switch kind {
			case "lock-symlink":
				err = os.Symlink(victim, lock)
			case "lock-hardlink":
				err = os.Link(victim, lock)
			case "lock-public":
				err = os.WriteFile(lock, nil, 0644)
			case "lock-directory":
				err = os.Mkdir(lock, 0700)
			case "parent-symlink":
				link := filepath.Join(t.TempDir(), "linked")
				err = os.Symlink(dir, link)
				writerPath = filepath.Join(link, "policy.json")
			case "parent-public":
				err = os.Chmod(dir, 0777)
			case "parent-missing":
				writerPath = filepath.Join(dir, "missing", "policy.json")
			}
			if err != nil {
				t.Fatal(err)
			}
			e := NewFilePolicyEvaluator(writerPath)
			for _, save := range []func(context.Context, core.CapabilityRequest, SavedChoice) error{e.Remember, e.RememberScope} {
				if err := save(context.Background(), persistenceRequest(), AllowEnvironment); err == nil {
					t.Fatal("saved authority through unsafe path")
				}
			}
			audit := &fakeAudit{}
			configuration := &PolicyConfiguration{Evaluator: e, Audit: audit}
			edit := PolicySnapshot{Revision: policyRevision(original), Policy: []byte(`{"default":"allow","rules":[]}`)}
			if _, err := configuration.Replace(context.Background(), edit); err == nil {
				t.Fatal("operator edit bypassed writer path validation")
			}
			if len(audit.events) != 0 {
				t.Fatal("unsafe path reached mutation audit")
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatal("failure changed existing Policy", err)
			}
			actual, err = os.ReadFile(victim)
			if err != nil || string(actual) != "preserve unrelated file" {
				t.Fatal("writer touched unrelated target", err)
			}
		})
	}
}

func TestPolicyPersistenceRefusesMalformedAndNonRegularSources(t *testing.T) {
	for _, kind := range []string{"malformed", "oversized", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			var original []byte
			var err error
			switch kind {
			case "malformed":
				original = []byte(`{"default":"deny"} trailing-invalid`)
				err = os.WriteFile(path, original, 0600)
			case "oversized":
				original = []byte(strings.Repeat(" ", maxSavedPolicyBytes+1))
				err = os.WriteFile(path, original, 0600)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			e := NewFilePolicyEvaluator(path)
			c := &PolicyConfiguration{Evaluator: e, Audit: &fakeAudit{}}
			if _, err := c.Snapshot(context.Background()); err == nil {
				t.Fatal("presented unsafe Policy as editable")
			}
			if err := e.Remember(context.Background(), persistenceRequest(), AllowEnvironment); err == nil {
				t.Fatal("replaced unsafe Policy with saved authority")
			}
			if original != nil {
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, original) {
					t.Fatal("unsafe input was overwritten", err)
				}
			}
			staged, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".policy-*"))
			if err != nil || (len(staged) != 1 || filepath.Base(staged[0]) != ".policy-save.lock") {
				t.Fatalf("rejected input left a staged Policy: %v, %v", staged, err)
			}
		})
	}
}

func TestPolicySizeAndLateCancellationLeaveNoPartialGrant(t *testing.T) {
	for _, failure := range []string{"encoded-size", "saved-size", "cancel-after-audit"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			original := []byte(`{"default":"deny","rules":[]}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			e := NewFilePolicyEvaluator(path)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var events []core.CapabilityAuditEvent
			c := &PolicyConfiguration{Evaluator: e, Audit: configurationAuditFunc(func(_ context.Context, event core.CapabilityAuditEvent) error {
				events = append(events, event)
				if failure == "cancel-after-audit" {
					cancel()
				}
				return nil
			})}
			edit, err := c.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "encoded-size":
				policy := PolicyFile{Default: core.PolicyAllow, Rules: []PolicyRule{{Capability: "local.echo", Action: "echo", Resource: "target", Decision: core.PolicyAllow}}}
				base, err := json.Marshal(policy)
				if err != nil {
					t.Fatal(err)
				}
				policy.Rules[0].Reason = strings.Repeat("x", maxSavedPolicyBytes-len(base)-20)
				edit.Policy, err = json.Marshal(policy)
				if err != nil || len(edit.Policy) > maxSavedPolicyBytes {
					t.Fatal("fixture must fit before canonical formatting", err)
				}
				_, err = c.Replace(ctx, edit)
				if !errors.Is(err, core.ErrInvalidArgument) {
					t.Fatal("expanded Policy exceeded durable size limit", err)
				}
			case "saved-size":
				request := persistenceRequest()
				request.Resource = strings.Repeat("x", maxSavedPolicyBytes)
				if err := e.Remember(ctx, request, AllowEnvironment); err == nil || !strings.Contains(err.Error(), "size limit") {
					t.Fatal("oversized saved choice was persisted", err)
				}
			case "cancel-after-audit":
				edit.Policy = []byte(`{"default":"allow","rules":[]}`)
				if _, err := c.Replace(ctx, edit); !errors.Is(err, context.Canceled) {
					t.Fatal("cancelled edit committed authority", err)
				}
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatal("failed edit changed existing authority", err)
			}
			for _, event := range events {
				if event.Type != "configuration-change-requested" {
					t.Fatal("failed edit emitted completion audit", event.Type)
				}
			}
			staged, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".policy-*"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range staged {
				if filepath.Base(file) != ".policy-save.lock" {
					t.Fatal("failed edit retained staged authority", file)
				}
			}
			if err := e.Remember(context.Background(), persistenceRequest(), DenyEnvironment); err != nil {
				t.Fatal("failure left writer lock held", err)
			}
		})
	}
}

func TestPolicyCancellationRetainsOriginalAndReleasesWriterLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	original := []byte(`{"default":"deny","rules":[]}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	e := NewFilePolicyEvaluator(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &PolicyConfiguration{Evaluator: e, Audit: &fakeAudit{}}
	if _, err := c.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("snapshot ignored cancellation", err)
	}
	if err := e.Remember(ctx, persistenceRequest(), AllowEnvironment); !errors.Is(err, context.Canceled) {
		t.Fatal("save ignored cancellation", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("cancelled save changed Policy", err)
	}
	if err := e.RememberScope(context.Background(), persistenceRequest(), DenyEnvironment); err != nil {
		t.Fatal("cancelled writer retained the lock", err)
	}
	decision, err := e.Evaluate(context.Background(), persistenceRequest())
	if err != nil || decision.Decision != core.PolicyDeny {
		t.Fatal("retry did not persist the explicit denial", decision, err)
	}
}

func TestInvalidPolicyRulesCannotReplaceOrWidenPersistedAuthority(t *testing.T) {
	valid := PolicyRule{Capability: "local.echo", Action: "echo", Resource: "target", Decision: core.PolicyAllow}
	mutations := map[string]func(*PolicyRule){
		"instance":          func(r *PolicyRule) { r.EnvironmentInstance = "not-an-instance" },
		"decision":          func(r *PolicyRule) { r.Decision = "approve" },
		"empty-capability":  func(r *PolicyRule) { r.Capability = " " },
		"control-character": func(r *PolicyRule) { r.Reason = "review\nforged output" },
		"empty-attribute":   func(r *PolicyRule) { r.Attributes = map[string]string{" ": "value"} },
		"attribute-control": func(r *PolicyRule) { r.Attributes = map[string]string{"scope": "target\x00other"} },
	}
	for name, mutate := range mutations {
		for _, saved := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/rule", true: "/saved"}[saved], func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "policy.json")
				e := NewFilePolicyEvaluator(path)
				c := &PolicyConfiguration{Evaluator: e, Audit: &fakeAudit{}}
				edit, err := c.Snapshot(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				rule := valid
				mutate(&rule)
				policy := PolicyFile{Default: core.PolicyAllow, Rules: []PolicyRule{rule}}
				if saved {
					policy.SavedDecisions, policy.Rules = policy.Rules, nil
				}
				edit.Policy, err = json.Marshal(policy)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := c.Replace(context.Background(), edit); !errors.Is(err, core.ErrInvalidArgument) {
					t.Fatal("invalid policy accepted", err)
				}
				decision, err := e.Evaluate(context.Background(), persistenceRequest())
				if err != nil || decision.Decision != core.PolicyDeny {
					t.Fatal("invalid edit widened authority", decision, err)
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("invalid edit created Policy", err)
				}
			})
		}
	}
}
