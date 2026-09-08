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

func TestSnapshotCLITransportAndFailedSaveHandle(t *testing.T) {
	server := control.NewServer()
	var got controlapi.SnapshotRequest
	calls := 0
	fail := false
	id := "snap-11111111111111111111111111111111"
	if err := server.Register(controlapi.MethodSnapshot, func(_ context.Context, payload json.RawMessage) (any, error) {
		calls++
		if err := json.Unmarshal(payload, &got); err != nil {
			return nil, err
		}
		result := map[string]any{"snapshots": []controlapi.SnapshotSummary{{ID: id, State: "ready", Environment: "demo", Workspaces: 2, OCI: true}}}
		if fail {
			result["error"] = map[string]any{"code": "recovery_required", "message": "saved but restart failed"}
		}
		return result, nil
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	for _, args := range [][]string{{"create", "demo"}, {"list"}, {"list", "--json", "demo"}, {"delete", id}} {
		code, out, diagnostic := captureRun(t, append([]string{"snapshot"}, args...)...)
		if code != 0 || diagnostic != "" || got.Operation != args[0] || out == "" {
			t.Fatal(code, out, diagnostic, got)
		}
	}
	fail = true
	code, out, diagnostic := captureRun(t, "snapshot", "create", "--json", "demo")
	var saved []controlapi.SnapshotSummary
	if code != 1 || !strings.Contains(diagnostic, "restart failed") || json.Unmarshal([]byte(out), &saved) != nil || len(saved) != 1 || saved[0].ID != id {
		t.Fatal(code, out, diagnostic)
	}
	before := calls
	for _, args := range [][]string{{}, {"create"}, {"delete"}, {"list", "a", "b"}, {"restore", "x"}, {"create", "--unknown", "demo"}} {
		code, _, _ := captureRun(t, append([]string{"snapshot"}, args...)...)
		if code != 2 {
			t.Fatal(args, code)
		}
	}
	for _, args := range [][]string{{"--help"}, {"create", "--help"}} {
		code, _, _ := captureRun(t, append([]string{"snapshot"}, args...)...)
		if code != 0 {
			t.Fatal(args, code)
		}
	}
	if calls != before {
		t.Fatal("usage reached controller")
	}
}
