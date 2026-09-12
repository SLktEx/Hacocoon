package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func TestEnvironmentCopyCLIRequiresOnlySourceAndPreservesFailureResult(t *testing.T) {
	server := control.NewServer()
	var request controlapi.EnvironmentCopyRequest
	fail := false
	if err := server.Register(controlapi.MethodEnvironmentCopy, func(_ context.Context, p json.RawMessage) (any, error) {
		if err := json.Unmarshal(p, &request); err != nil {
			return nil, err
		}
		result := map[string]any{"result": map[string]string{"environment": "dev-restored", "workspace": "restore-owned", "state": "running"}}
		if fail {
			result["error"] = map[string]string{"code": "recovery_required", "message": "owned residue retained"}
		}
		return result, nil
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "restore.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	id := "dev"
	code, out, diagnostic := captureRun(t, "env", "copy", id)
	if code != 0 || request.Source != id || request.Target != "" || !strings.Contains(out, "dev-restored") || diagnostic != "" {
		t.Fatal(code, out, diagnostic, request)
	}
	fail = true
	code, out, diagnostic = captureRun(t, "env", "copy", "--json", id, "new")
	var result map[string]string
	if code != 1 || json.Unmarshal([]byte(out), &result) != nil || result["workspace"] != "restore-owned" || request.Target != "new" || !strings.Contains(diagnostic, "residue") {
		t.Fatal(code, out, diagnostic)
	}
}
