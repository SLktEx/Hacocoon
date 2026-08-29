package capability

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestStdioApprovalEscapesTerminalControlSequences(t *testing.T) {
	var out bytes.Buffer
	approval := NewStdioApproval(strings.NewReader("n\n"), &out)
	req := core.ApprovalRequest{
		CapabilityRequest: core.CapabilityRequest{
			Capability:  "local.echo",
			Action:      "echo",
			Resource:    "safe\x1b[2Jfake-approved-resource",
			Environment: "demo\x1b[H",
			Attributes:  map[string]string{"target": "real\x1b[2Kfake"},
		},
		Reason: "review\x1b[2JALLOW",
	}

	approved, err := approval.Approve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if approved {
		t.Fatal("unexpected approval")
	}
	shown := out.String()
	if strings.ContainsRune(shown, '\x1b') {
		t.Fatalf("raw terminal escape reached approval UI: %q", shown)
	}
	if !strings.Contains(shown, `\x1b[2J`) {
		t.Fatalf("escaped control sequence was not rendered visibly: %q", shown)
	}
}
