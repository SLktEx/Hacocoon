package capability

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func TestTerminalSavedChoicesAreExplicit(t *testing.T) {
	for input, want := range map[string]SavedChoice{"1": AllowEnvironment, "2": DenyEnvironment, "3": AllowGlobal, "4": DenyGlobal} {
		var out bytes.Buffer
		approval := NewStdioApproval(strings.NewReader(input+"\n"), &out)
		decision, err := approval.Decide(context.Background(), core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}})
		if err != nil || decision.Save != want || decision.Approved != (want == AllowEnvironment || want == AllowGlobal) {
			t.Fatalf("%s: %#v %v", input, decision, err)
		}
		if !strings.Contains(out.String(), "allow all Environments") || !strings.Contains(out.String(), "deny this Environment") {
			t.Fatal("scope missing")
		}
	}
}
func TestLegacyApprovalCannotAccidentallySave(t *testing.T) {
	approved, err := NewStdioApproval(strings.NewReader("1\n"), &bytes.Buffer{}).Approve(context.Background(), core.ApprovalRequest{})
	if err != nil || approved {
		t.Fatal("legacy numeric input granted authority")
	}
}
