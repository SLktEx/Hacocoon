package capability

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestJSONLAuditUsesPrivateFileAndOmitsParametersByType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "capabilities.jsonl")
	audit := NewJSONLAudit(path)
	event := core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), Type: "requested", Capability: "local.echo", Action: "echo", Resource: "safe"}
	if err := audit.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("audit file permissions are too broad: %o", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"capability":"local.echo"`) || strings.Contains(string(content), "parameters") {
		t.Fatalf("audit content=%s", content)
	}
}
