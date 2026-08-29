package capability

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestFilePolicyMatchesEveryAuditableAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	policy := `{"default":"deny","rules":[{"capability":"github.git","action":"push","resource":"*","environment":"demo","attributes":{"organization":"acme","repository":"demo","target_ref":"refs/heads/feature/x","source_sha":"*"},"decision":"allow"}]}`
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator := NewFilePolicyEvaluator(path)
	for _, tc := range []struct {
		name  string
		attrs map[string]string
		want  core.PolicyDecision
	}{
		{name: "exact scope", attrs: map[string]string{"organization": "acme", "repository": "demo", "target_ref": "refs/heads/feature/x", "source_sha": "abc"}, want: core.PolicyAllow},
		{name: "wrong repo", attrs: map[string]string{"organization": "acme", "repository": "other", "target_ref": "refs/heads/feature/x", "source_sha": "abc"}, want: core.PolicyDeny},
		{name: "extra authority input", attrs: map[string]string{"organization": "acme", "repository": "demo", "target_ref": "refs/heads/feature/x", "source_sha": "abc", "unexpected": "authority"}, want: core.PolicyDeny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(context.Background(), core.CapabilityRequest{Capability: "github.git", Action: "push", Environment: "demo", Attributes: tc.attrs})
			if err != nil || got.Decision != tc.want {
				t.Fatalf("got=%#v err=%v", got, err)
			}
		})
	}
}

func TestApprovalPromptShowsSortedSafeAttributes(t *testing.T) {
	var out bytes.Buffer
	approved, err := NewStdioApproval(strings.NewReader("yes\n"), &out).Approve(context.Background(), core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{
		Capability: "github.git", Action: "push", Resource: "github://acme/demo/refs/heads/feature/x", Environment: "demo",
		Attributes: map[string]string{"source_sha": "deadbeef", "repository": "demo", "organization": "acme"},
	}, Reason: "policy requires approval"})
	if err != nil || !approved {
		t.Fatalf("approved=%t err=%v", approved, err)
	}
	prompt := out.String()
	for _, want := range []string{"environment=demo", "organization=acme", "repository=demo", "source_sha=deadbeef", "reason=policy requires approval"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in %q", want, prompt)
		}
	}
	if strings.Index(prompt, "organization=") > strings.Index(prompt, "repository=") || strings.Index(prompt, "repository=") > strings.Index(prompt, "source_sha=") {
		t.Fatalf("attributes not sorted: %q", prompt)
	}
}

func TestAuditIncludesAttributesButStillExcludesDeclaredNonAuthorityParameters(t *testing.T) {
	audit := &fakeAudit{}
	provider := &fakeProvider{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, &fakeApproval{}, audit, provider)
	_, err := service.Request(context.Background(), core.CapabilityRequest{
		Capability: "local.echo", Action: "echo",
		Attributes: map[string]string{"repository": "demo", "source_sha": "abc"},
		Parameters: map[string]string{"message": "secret-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range audit.events {
		if event.Attributes["repository"] != "demo" || event.Attributes["source_sha"] != "abc" {
			t.Fatalf("attributes missing: %#v", event)
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(event.Reason)), "secret-value") {
			t.Fatalf("secret leaked into audit: %#v", event)
		}
	}
}
