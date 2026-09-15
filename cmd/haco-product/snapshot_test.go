package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSnapshotCLITransportAndFailedSaveHandle(t *testing.T) {
	server := control.NewServer()
	var got controlapi.SnapshotRequest
	calls := 0
	fail := false
	id := "snap-11111111111111111111111111111111"
	if err := server.Register(controlapi.MethodSnapshot, func(_ context.Context, payload json.RawMessage) (any, error) {
		calls++
		got = controlapi.SnapshotRequest{}
		if err := json.Unmarshal(payload, &got); err != nil {
			return nil, err
		}
		result := map[string]any{"snapshots": []controlapi.SnapshotSummary{{ID: id, State: "ready", Environment: "demo", Workspaces: 2, OCI: true}}}
		if got.Operation == "inspect" {
			n := 1
			result["inspection"] = core.SnapshotInspection{ID: id, Environment: "demo", State: "deleting", Partial: fail, Components: []core.SnapshotComponentInspection{{Role: "workspace:main", State: "verified", Presence: "present", Check: "busy", References: &n, Provider: "incus", Pool: "pool", Object: "private-object", Backing: "uninspected"}}}
		}
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
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		code, out, diagnostic := captureRun(t, "snapshot", "inspect", id)
		if code != 0 || diagnostic != "" || got.ID != id || got.Environment != "" || strings.Contains(out, "private-object") || !strings.Contains(out, "haco snapshot delete "+id) {
			t.Fatal(code, out, diagnostic, got)
		}
		want := "Still referenced"
		if language == "ja" {
			want = "使用中です"
		}
		if !strings.Contains(out, want) {
			t.Fatal("untranslated inspection", out)
		}
		code, out, diagnostic = captureRun(t, "snapshot", "inspect", "--details", id)
		if code != 0 || !strings.Contains(out, "private-object") || diagnostic != "" {
			t.Fatal(code, out, diagnostic)
		}
	}
	fail = true
	codeJSON, outJSON, diagnosticJSON := captureRun(t, "snapshot", "inspect", "--json", id)
	var inspection core.SnapshotInspection
	if codeJSON != 1 || diagnosticJSON == "" || json.Unmarshal([]byte(outJSON), &inspection) != nil || !inspection.Partial || inspection.ID != id {
		t.Fatal(codeJSON, outJSON, diagnosticJSON)
	}
	codeDelete, _, diagnosticDelete := captureRun(t, "snapshot", "delete", id)
	if codeDelete != 1 || !strings.Contains(diagnosticDelete, "haco snapshot inspect "+id) {
		t.Fatal(codeDelete, diagnosticDelete)
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
