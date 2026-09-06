package composition

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestControllerEgressPolicyDoesNotReadAmbientApproval(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HACO_ROOT", root)
	t.Setenv("HACO_PLUGIN_OCI", "")
	t.Setenv("HACO_RUNTIME_PROVIDER", "")
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString("yes\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = original })
	app, err := Controller(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	request := core.CapabilityRequest{Capability: "network.egress", Action: "connect", Resource: "example.com", Environment: "env-a", Attributes: map[string]string{"protocol": "https", "port": "443"}}
	for _, decision := range []string{"missing", "require-approval", "deny", "allow"} {
		if decision != "missing" {
			policy := `{"default":"deny","rules":[{"capability":"network.egress","action":"connect","resource":"example.com","environment":"env-a","attributes":{"protocol":"https","port":"443"},"decision":"` + decision + `"}]}`
			if err := os.WriteFile(filepath.Join(root, "policy.json"), []byte(policy), 0600); err != nil {
				t.Fatal(err)
			}
		}
		result, err := app.Capabilities.Request(context.Background(), request)
		if decision == "allow" {
			if err != nil || !result.AuditComplete || result.ExecutionState != core.CapabilitySucceeded {
				t.Fatalf("allow: %+v %v", result, err)
			}
		} else if decision == "require-approval" {
			if !errors.Is(err, core.ErrApprovalDenied) {
				t.Fatalf("headless approval: %v", err)
			}
		} else if !errors.Is(err, core.ErrPolicyDenied) {
			t.Fatalf("%s: %v", decision, err)
		}
	}
	if position, err := input.Seek(0, 1); err != nil || position != 0 {
		t.Fatalf("daemon consumed ambient stdin: %d %v", position, err)
	}
	request.Resource = "other.example"
	if _, err := app.Capabilities.Request(context.Background(), request); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("allow expanded to another host: %v", err)
	}
	// The actual controller composition persisted audit events, not a test stub.
	data, err := os.ReadFile(filepath.Join(root, "audit", "capabilities.jsonl"))
	if err != nil || len(data) == 0 {
		t.Fatalf("audit unavailable: %v", err)
	}
}
