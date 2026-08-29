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
	service, err := capabilityapp.New(
		capabilityapp.NewFilePolicyEvaluator(policyPath),
		capabilityapp.NewStdioApproval(strings.NewReader("yes\n"), &prompt),
		capabilityapp.NewJSONLAudit(auditPath),
		capabilityapp.LocalEcho{},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Request(context.Background(), core.CapabilityRequest{
		Capability: "local.echo", Action: "echo", Resource: "sensitive", Parameters: map[string]string{"message": "private-parameter"},
	})
	if err != nil || result.Output != "private-parameter" || result.RequestID == "" || result.ExecutionState != core.CapabilitySucceeded || !result.AuditComplete {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(prompt.String(), "Approve capability local.echo") || !strings.Contains(prompt.String(), "reason=sensitive local action") {
		t.Fatalf("prompt=%q", prompt.String())
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(audit)
	for _, want := range []string{`"type":"requested"`, `"decision":"require-approval"`, `"approved":true`, `"success":true`, `"request_id":"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in audit:\n%s", want, text)
		}
	}
	if strings.Contains(text, "private-parameter") {
		t.Fatalf("non-authority parameter leaked into audit:\n%s", text)
	}
}
