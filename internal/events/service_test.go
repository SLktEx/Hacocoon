package events

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestListExportsSecurityApprovalEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	content := "" +
		`{"time":"2026-08-29T08:00:00Z","request_id":"req-1","type":"policy-decision","capability":"github.git","action":"force-push","resource":"github://acme/demo/refs/heads/main","environment":"demo","attributes":{"organization":"acme","source_sha":"abc"},"decision":"require-approval","reason":"protected"}` + "\n" +
		`{"time":"2026-08-29T08:00:01Z","request_id":"req-1","type":"approval-decision","capability":"github.git","action":"force-push","environment":"demo","approved":true}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := New(path).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].RequestID != "req-1" || events[0].Source != "capability" || events[0].Decision != core.PolicyRequireApproval || events[0].Attributes["organization"] != "acme" {
		t.Fatalf("events=%#v", events)
	}
	if events[1].Approved == nil || !*events[1].Approved {
		t.Fatalf("approval event=%#v", events[1])
	}
}

func TestListMissingAuditIsEmpty(t *testing.T) {
	events, err := New(filepath.Join(t.TempDir(), "missing.jsonl")).List(context.Background())
	if err != nil || len(events) != 0 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestListRejectsMalformedOrIncompleteAudit(t *testing.T) {
	for _, content := range []string{"not-json\n", `{"type":"requested"}` + "\n"} {
		path := filepath.Join(t.TempDir(), "audit.jsonl")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := New(path).List(context.Background())
		if err == nil {
			t.Fatalf("content=%q unexpectedly accepted", content)
		}
	}
}

func TestListHonorsCancelledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(`{"time":"2026-08-29T08:00:00Z","type":"requested"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(path).List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
