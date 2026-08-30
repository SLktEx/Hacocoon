package events

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		events, err := New(path).List(context.Background())
		if err == nil || len(events) != 0 {
			t.Fatalf("content=%q events=%#v err=%v", content, events, err)
		}
		var corruption *AuditCorruptionError
		if !errors.As(err, &corruption) || corruption.Line != 1 || corruption.ByteOffset != 0 {
			t.Fatalf("content=%q corruption=%#v err=%v", content, corruption, err)
		}
	}
}

func TestListReturnsTrustworthyPrefixAndStopsAtFirstMalformedRecord(t *testing.T) {
	first := `{"time":"2026-08-29T08:00:00Z","request_id":"req-before","type":"requested","capability":"github.git"}`
	malformed := `{"time":"2026-08-29T08:00:01Z","type":`
	after := `{"time":"2026-08-29T08:00:02Z","request_id":"req-after","type":"completed"}`
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	content := first + "\n" + malformed + "\n" + after + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := New(path).List(context.Background())
	if len(events) != 1 || events[0].RequestID != "req-before" {
		t.Fatalf("events=%#v", events)
	}
	var corruption *AuditCorruptionError
	if !errors.As(err, &corruption) {
		t.Fatalf("expected AuditCorruptionError, got %v", err)
	}
	if corruption.Line != 2 || corruption.ByteOffset != int64(len(first)+1) || corruption.Kind != CorruptionMalformedJSON {
		t.Fatalf("corruption=%#v", corruption)
	}
	for _, event := range events {
		if event.RequestID == "req-after" {
			t.Fatalf("record after corruption must not be exposed: %#v", events)
		}
	}
}

func TestListReturnsTrustworthyPrefixOnIncompleteRecord(t *testing.T) {
	first := `{"time":"2026-08-29T08:00:00Z","request_id":"req-before","type":"requested"}`
	incomplete := `{"request_id":"broken","type":"completed"}`
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(first+"\n"+incomplete+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := New(path).List(context.Background())
	if len(events) != 1 || events[0].RequestID != "req-before" {
		t.Fatalf("events=%#v", events)
	}
	var corruption *AuditCorruptionError
	if !errors.As(err, &corruption) || corruption.Kind != CorruptionIncomplete || corruption.Line != 2 || corruption.ByteOffset != int64(len(first)+1) {
		t.Fatalf("corruption=%#v err=%v", corruption, err)
	}
}

func TestListReturnsTrustworthyPrefixOnOversizedRecord(t *testing.T) {
	first := `{"time":"2026-08-29T08:00:00Z","request_id":"req-before","type":"requested"}`
	oversized := strings.Repeat("x", 1024*1024+1)
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(first+"\n"+oversized+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := New(path).List(context.Background())
	if len(events) != 1 || events[0].RequestID != "req-before" {
		t.Fatalf("events=%#v", events)
	}
	var corruption *AuditCorruptionError
	if !errors.As(err, &corruption) || corruption.Kind != CorruptionReadError || corruption.Line != 2 || corruption.ByteOffset != int64(len(first)+1) {
		t.Fatalf("corruption=%#v err=%v", corruption, err)
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
