//go:build linux

package capability

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type configurationAuditFunc func(context.Context, core.CapabilityAuditEvent) error

func TestConfigurationRoundTripKeepsCanonicalViewOfEmptySavedChoices(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "policy.json")
	raw := []byte(`{"default":"deny","rules":[],"saved_decisions":[]}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c := &PolicyConfiguration{Evaluator: NewFilePolicyEvaluator(path), Audit: &fakeAudit{}}
	before, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Revision != policyRevision(raw) {
		t.Fatal("revision no longer binds to raw bytes")
	}
	after, err := c.Replace(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	var first, second any
	if json.Unmarshal(before.Policy, &first) != nil || json.Unmarshal(after.Policy, &second) != nil {
		t.Fatal("invalid view")
	}
	x, _ := json.Marshal(first)
	y, _ := json.Marshal(second)
	if string(x) != string(y) {
		t.Fatalf("empty saved choices changed display: %s -> %s", x, y)
	}
}

func (f configurationAuditFunc) Record(ctx context.Context, event core.CapabilityAuditEvent) error {
	return f(ctx, event)
}

func TestConfigurationAndSavedChoiceShareWriteLock(t *testing.T) {
	ctx := context.Background()
	e := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
	entered, release := make(chan struct{}), make(chan struct{})
	c := &PolicyConfiguration{Evaluator: e, Audit: configurationAuditFunc(func(_ context.Context, event core.CapabilityAuditEvent) error {
		if event.Type == "configuration-change-requested" {
			close(entered)
			<-release
		}
		return nil
	})}
	edit, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := c.Replace(ctx, edit); done <- err }()
	<-entered
	req := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}
	saveErr := e.Remember(ctx, req, AllowEnvironment)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if saveErr == nil {
		t.Fatal("saved choice bypassed configuration writer lock")
	}
	if err := e.Remember(ctx, req, AllowEnvironment); err != nil {
		t.Fatal(err)
	}
	policy, err := e.load()
	if err != nil || policy.Default != core.PolicyDeny || len(policy.SavedDecisions) != 1 {
		t.Fatal("subsequent saved choice lost committed policy")
	}
}

func TestConfigurationDetectsUncoordinatedFileChange(t *testing.T) {
	e := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
	external := []byte(`{"default":"require-approval","rules":[]}`)
	c := &PolicyConfiguration{Evaluator: e, Audit: configurationAuditFunc(func(_ context.Context, event core.CapabilityAuditEvent) error {
		if event.Type != "configuration-change-requested" {
			t.Error("unexpected commit acknowledgement")
		}
		return os.WriteFile(e.path, external, 0600)
	})}
	edit, err := c.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Replace(context.Background(), edit); err == nil {
		t.Fatal("overwrote external change")
	}
	actual, err := os.ReadFile(e.path)
	if err != nil || string(actual) != string(external) {
		t.Fatal("external change lost")
	}
}

func TestConfigurationEditConflictsWithSavedApproval(t *testing.T) {
	ctx := context.Background()
	e := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
	audit := &fakeAudit{}
	c := &PolicyConfiguration{Evaluator: e, Audit: audit}
	initial, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	req := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}
	if err := e.Remember(ctx, req, AllowEnvironment); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Replace(ctx, initial); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	if len(audit.events) != 0 {
		t.Fatal("stale edit reached mutation audit")
	}
	fresh, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var policy PolicyFile
	if err := json.Unmarshal(fresh.Policy, &policy); err != nil {
		t.Fatal(err)
	}
	if len(policy.SavedDecisions) != 1 {
		t.Fatal("saved approval was lost")
	}
	policy.SavedDecisions[0].Decision = core.PolicyDeny
	fresh.Policy, _ = json.Marshal(policy)
	result, err := c.Replace(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := c.Snapshot(ctx)
	if err != nil || observed.Revision != result.Revision || result.Revision == fresh.Revision {
		t.Fatal("receipt does not match persisted configuration")
	}
	evaluation, err := e.Evaluate(ctx, req)
	if err != nil || evaluation.Decision != core.PolicyDeny {
		t.Fatalf("edited rule not applied to next request: %#v %v", evaluation, err)
	}
	if len(audit.events) != 2 || audit.events[0].Type != "configuration-change-requested" || audit.events[1].Type != "configuration-changed" {
		t.Fatal("missing configuration audit")
	}
	encoded, _ := json.Marshal(audit.events)
	if strings.Contains(string(encoded), "target") || strings.Contains(string(encoded), req.EnvironmentInstance) {
		t.Fatal("configuration content leaked into audit")
	}
}

func TestConfigurationRefusesInvalidInputAndAuditFailures(t *testing.T) {
	for _, invalid := range []string{`{"default":"typo","rules":[]}`, `{"default":"deny","unknown":"secret"}`, `{"default":"deny"} {}`, "null", strings.Repeat(" ", maxSavedPolicyBytes+1)} {
		t.Run("invalid", func(t *testing.T) {
			e := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
			c := &PolicyConfiguration{Evaluator: e, Audit: &fakeAudit{}}
			edit, err := c.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			edit.Policy = []byte(invalid)
			if _, err := c.Replace(context.Background(), edit); err == nil {
				t.Fatal("invalid edit accepted")
			}
			if _, err := os.Stat(e.path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid edit created policy")
			}
		})
	}
	for _, failAt := range []int{1, 2} {
		e := NewFilePolicyEvaluator(filepath.Join(t.TempDir(), "policy.json"))
		c := &PolicyConfiguration{Evaluator: e, Audit: &fakeAudit{failAt: failAt}}
		edit, err := c.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		result, err := c.Replace(context.Background(), edit)
		if err == nil || result.Revision != "" {
			t.Fatal("audit failure acknowledged")
		}
		_, statErr := os.Stat(e.path)
		if failAt == 1 && !errors.Is(statErr, os.ErrNotExist) {
			t.Fatal("pre-write audit failure mutated policy")
		}
		if failAt == 2 && (statErr != nil || !errors.Is(err, core.ErrRecoveryRequired)) {
			t.Fatal("post-write failure hid committed state")
		}
	}
}

func TestConfigurationSnapshotRefusesUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "public"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.json")
			target := filepath.Join(dir, "target")
			if err := os.WriteFile(target, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "hardlink":
				err = os.Link(target, path)
			case "public":
				err = os.WriteFile(path, []byte(`{}`), 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			c := &PolicyConfiguration{Evaluator: NewFilePolicyEvaluator(path)}
			if _, err := c.Snapshot(context.Background()); err == nil {
				t.Fatal("unsafe snapshot read")
			}
		})
	}
}
