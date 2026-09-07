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

func TestTerminalSavedAskStillRequiresOneShotAnswer(t *testing.T) {
	for _, tc := range []struct {
		input    string
		save     SavedChoice
		approved bool
	}{
		{"5\nyes\n", AskEnvironment, true},
		{"5\nno\n", AskEnvironment, false},
		{"6\nyes\n", AskGlobal, true},
		{"6\n", AskGlobal, false},
	} {
		var out bytes.Buffer
		decision, err := NewStdioApproval(strings.NewReader(tc.input), &out).Decide(context.Background(), core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}})
		if err != nil || decision.Save != tc.save || decision.Approved != tc.approved {
			t.Fatalf("%q: %#v %v", tc.input, decision, err)
		}
		if !strings.Contains(out.String(), "[y/N]") {
			t.Fatal("ask did not request one-shot approval")
		}
	}
}
