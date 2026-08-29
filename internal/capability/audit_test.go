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
	event := core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), RequestID: "request-1", Type: "requested", Capability: "local.echo", Action: "echo", Resource: "safe"}
	if err := audit.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("audit file permissions=%o want 600", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"capability":"local.echo"`) || !strings.Contains(string(content), `"request_id":"request-1"`) || strings.Contains(string(content), "parameters") {
		t.Fatalf("audit content=%s", content)
	}
}

func TestJSONLAuditTightensExistingFileAndDirectoryPermissions(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "audit")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "capabilities.jsonl")
	if err := os.WriteFile(path, []byte("preexisting\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewJSONLAudit(path).Record(context.Background(), core.CapabilityAuditEvent{RequestID: "request-2", Type: "requested", Capability: "local.echo", Action: "echo"}); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("file=%o dir=%o", fileInfo.Mode().Perm(), dirInfo.Mode().Perm())
	}
}
