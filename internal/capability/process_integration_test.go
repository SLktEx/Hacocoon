package capability_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCapabilityFlowAcrossFilePolicyApprovalAndAudit(t *testing.T) {
	root := t.TempDir()
	policyPath := filepath.Join(root, "policy.json")
	auditPath := filepath.Join(root, "audit", "capabilities.jsonl")
	policy := `{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"sensitive","decision":"require-approval","reason":"sensitive local action"}]}`
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	var prompt bytes.Buffer
	service := capabilityapp.New(
		capabilityapp.NewFilePolicyEvaluator(policyPath),
		capabilityapp.NewStdioApproval(strings.NewReader("yes\n"), &prompt),
		capabilityapp.NewJSONLAudit(auditPath),
		capabilityapp.LocalEcho{},
	)
	result, err := service.Request(context.Background(), core.CapabilityRequest{
		Capability: "local.echo", Action: "echo", Resource: "sensitive", Parameters: map[string]string{"message": "private-parameter"},
	})
	if err != nil || result.Output != "private-parameter" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(prompt.String(), "Approve capability local.echo") {
		t.Fatalf("prompt=%q", prompt.String())
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(audit)
	for _, want := range []string{`"type":"requested"`, `"decision":"require-approval"`, `"approved":true`, `"success":true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in audit:\n%s", want, text)
		}
	}
	if strings.Contains(text, "private-parameter") {
		t.Fatalf("request parameter leaked into audit:\n%s", text)
	}
}
