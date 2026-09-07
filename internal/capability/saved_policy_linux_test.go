//go:build linux

package capability

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestRememberPreservesRestrictionsAndUpdatesOnlySavedDecisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	original := []byte(`{"default":"require-approval","rules":[{"capability":"local.echo","action":"echo","resource":"blocked","environment":"*","decision":"deny"}]}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}
	if err := evaluator.Remember(context.Background(), request, AllowEnvironment); err != nil {
		t.Fatal(err)
	}
	if err := evaluator.Remember(context.Background(), request, DenyEnvironment); err != nil {
		t.Fatal(err)
	}
	policy, err := evaluator.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Rules) != 1 || policy.Rules[0].Resource != "blocked" || len(policy.SavedDecisions) != 1 {
		t.Fatal("administrator rule lost or saved duplicates")
	}
	got, err := evaluator.Evaluate(context.Background(), request)
	if err != nil || got.Decision != core.PolicyDeny {
		t.Fatal("saved denial ineffective")
	}
	request.Resource = "blocked"
	if err = evaluator.Remember(context.Background(), request, AllowGlobal); err != nil {
		t.Fatal(err)
	}
	got, err = evaluator.Evaluate(context.Background(), request)
	if err != nil || got.Decision != core.PolicyDeny {
		t.Fatal("remembered grant bypassed administrator denial")
	}
}
func TestRememberRefusesUnsafePolicyFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "public"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "policy.json")
			target := filepath.Join(root, "target")
			original := []byte(`{"default":"deny","rules":[]}`)
			if err := os.WriteFile(target, original, 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "hardlink":
				err = os.Link(target, path)
			case "public":
				err = os.WriteFile(path, original, 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}
			if err := NewFilePolicyEvaluator(path).Remember(context.Background(), request, AllowEnvironment); err == nil {
				t.Fatal("unsafe file accepted")
			}
			after, err := os.ReadFile(target)
			if err != nil || string(after) != string(original) {
				t.Fatal("target changed")
			}
		})
	}
}
