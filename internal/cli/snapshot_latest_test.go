package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
)

func TestSnapshotLatestUsesRecordedTimeAndNeverFallsBackAfterFailure(t *testing.T) {
	server := control.NewServer()
	older := controlapi.SnapshotSummary{ID: "snap-" + strings.Repeat("f", 32), Environment: "dev", State: "ready", CreatedAt: time.Date(2026, 9, 15, 1, 0, 0, 0, time.UTC)}
	newest := older
	newest.ID = "snap-" + strings.Repeat("a", 32)
	newest.CreatedAt = newest.CreatedAt.Add(time.Hour)
	incomplete := newest
	incomplete.ID = "snap-" + strings.Repeat("b", 32)
	incomplete.State = "recovery-required"
	incomplete.CreatedAt = incomplete.CreatedAt.Add(time.Hour)
	other := incomplete
	other.Environment = "other"
	other.State = "ready"
	saves := []controlapi.SnapshotSummary{newest, other, older, incomplete}
	listFailed, restoreFailed := false, false
	calls, lists := 0, 0
	var got controlapi.SnapshotRestoreRequest
	if err := server.Register(controlapi.MethodSnapshot, func(_ context.Context, p json.RawMessage) (any, error) {
		lists++
		var request controlapi.SnapshotRequest
		if err := json.Unmarshal(p, &request); err != nil {
			return nil, err
		}
		if request.Operation != "list" || request.Environment != "dev" || request.ID != "" {
			t.Error("wrong discovery request", request)
		}
		result := map[string]any{"snapshots": saves}
		if listFailed {
			result["error"] = map[string]string{"code": "unavailable", "message": "list unavailable"}
		}
		return result, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.Register(controlapi.MethodSnapshotRestore, func(_ context.Context, p json.RawMessage) (any, error) {
		calls++
		if err := json.Unmarshal(p, &got); err != nil {
			return nil, err
		}
		result := map[string]any{"result": map[string]string{"environment": "restored", "workspace": "owned", "state": "running"}}
		if restoreFailed {
			result["error"] = map[string]string{"code": "not_found", "message": "selected save disappeared"}
		}
		return result, nil
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "latest.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	for _, lang := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", lang)
		before := calls
		code, out, diagnostic := captureRun(t, "snapshot", "restore", "--latest", "dev", "restored")
		if code != 0 || calls != before+1 || got.ID != newest.ID || got.Environment != "restored" || !strings.Contains(out, "restored") || !strings.Contains(diagnostic, newest.ID) {
			t.Fatal(code, out, diagnostic, got)
		}
	}
	before := calls
	beforeLists := lists
	restoreFailed = true
	code, out, diagnostic := captureRun(t, "snapshot", "restore", "--json", "--latest", "dev", "restored")
	var result map[string]string
	if code != 1 || calls != before+1 || lists != beforeLists+1 || got.ID != newest.ID || json.Unmarshal([]byte(out), &result) != nil || result["workspace"] != "owned" || !strings.Contains(diagnostic, "disappeared") {
		t.Fatal(code, out, diagnostic, got)
	}
	restoreFailed = false
	for _, kind := range []string{"list-failed", "none", "undated", "tied"} {
		t.Run(kind, func(t *testing.T) {
			listFailed = kind == "list-failed"
			saves = []controlapi.SnapshotSummary{newest}
			switch kind {
			case "none":
				saves = nil
			case "undated":
				undated := older
				undated.CreatedAt = time.Time{}
				saves = append(saves, undated)
			case "tied":
				tied := older
				tied.CreatedAt = newest.CreatedAt
				saves = append(saves, tied)
			}
			before := calls
			code, _, diagnostic := captureRun(t, "snapshot", "restore", "--latest", "dev")
			if code != 1 || calls != before || diagnostic == "" {
				t.Fatal("uncertain choice restored", code, diagnostic)
			}
		})
	}
}
