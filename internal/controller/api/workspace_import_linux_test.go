//go:build linux

package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

func TestWorkspaceImportTransportUsesBytesAndRetainsOwnership(t *testing.T) {
	for _, failure := range []bool{false, true} {
		calls := 0
		socket := doctorTestSocket(t, func(s *control.Server) {
			if err := RegisterWorkspaceImport(s, func(_ context.Context, r io.Reader, req WorkspaceImportRequest) (WorkspaceImportResult, error) {
				calls++
				raw, err := io.ReadAll(r)
				if err != nil {
					return WorkspaceImportResult{}, err
				}
				if string(raw) != "portable tree bytes" || req.Repository != "source" {
					t.Error("wrong source")
				}
				result := WorkspaceImportResult{Reference: workflow.Reference{Name: req.Name, Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32))}, Repository: req.Repository, State: "ready"}
				if failure {
					result.State = "recovery-required"
					return result, core.ErrRecoveryRequired
				}
				return result, nil
			}); err != nil {
				t.Fatal(err)
			}
		})
		c, err := NewClient(socket)
		if err != nil {
			t.Fatal(err)
		}
		result, err := c.ImportWorkspace(context.Background(), strings.NewReader("portable tree bytes"), WorkspaceImportRequest{Name: "new-work", Repository: "source"})
		if (err != nil) != failure || result.Workspace == "" || calls != 1 {
			t.Fatal(result, err, calls)
		}
		for _, raw := range []string{`{"name":"work","repository":"source","path":"/etc/passwd"}`, `{"name":"../work","repository":"source"}`, `{"name":"work","repository":"source","remote":"https://example.invalid"}`} {
			conn, err := c.wire.OpenStream(context.Background(), MethodWorkspaceImport, json.RawMessage(raw))
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil {
				t.Fatal("unsafe request accepted")
			}
		}
		if calls != 1 {
			t.Fatal("invalid input reached operation")
		}
	}
}

func TestWorkspaceImportRejectsForeignResultIdentity(t *testing.T) {
	for _, id := range []core.WorkspaceID{"foreign:" + core.WorkspaceID(strings.Repeat("a", 32)), "workspace:managed:", "workspace:managed:unsafe"} {
		if validWorkspaceImportResult(WorkspaceImportResult{Reference: workflow.Reference{Workspace: id}, State: "ready"}) {
			t.Fatal("unsafe result accepted", id)
		}
	}
	if _, err := (&Client{}).ImportWorkspace(context.Background(), bytes.NewReader(nil), WorkspaceImportRequest{}); err == nil {
		t.Fatal("invalid request accepted")
	}
}
