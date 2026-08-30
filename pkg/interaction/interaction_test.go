package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func writeAudit(t *testing.T, root string, events ...core.CapabilityAuditEvent) string {
	t.Helper()
	path := filepath.Join(root, "audit", "capabilities.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, event := range events {
		if err := enc.Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func boolp(v bool) *bool { return &v }

func TestProjectionMinimizesSensitiveFields(t *testing.T) {
	root := t.TempDir()
	writeAudit(t, root, core.CapabilityAuditEvent{
		Time: time.Unix(10, 0).UTC(), RequestID: "req-1", Type: "policy-decision",
		Capability: "git", Action: "push", Environment: "env-a", Decision: core.PolicyRequireApproval,
		Resource: "https://user:secret@example.invalid/repo", Attributes: map[string]string{"authorization": "Bearer TOPSECRET"}, Reason: "TOPSECRET reason",
	})
	reader, err := NewReader(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reader.Batch(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.Events[0].Kind != ApprovalRequired {
		t.Fatalf("batch=%#v", batch)
	}
	payload, _ := json.Marshal(batch)
	for _, forbidden := range []string{"TOPSECRET", "authorization", "Bearer", "example.invalid", "resource", "attributes", "reason"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("client payload leaked %q: %s", forbidden, payload)
		}
	}
}

func TestRecoveryRequiredUsesClosedCodeSet(t *testing.T) {
	root := t.TempDir()
	writeAudit(t, root,
		core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), RequestID: "req-a", Type: "completed", Capability: "storage", Action: "delete", Success: boolp(false), Reason: "recovery-required"},
		core.CapabilityAuditEvent{Time: time.Unix(2, 0).UTC(), RequestID: "req-b", Type: "completed", Capability: "git", Action: "push", Success: boolp(false), Reason: "token=SHOULD-NOT-LEAK"},
	)
	reader, _ := NewReader(root)
	batch, err := reader.Batch(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := batch.Events[0]; got.Kind != RecoveryRequired || !got.RecoveryRequired || got.Code != "recovery-required" {
		t.Fatalf("first=%#v", got)
	}
	if got := batch.Events[1]; got.Kind != OperationFailed || got.Code != "operation-failed" || strings.Contains(got.Code, "SHOULD") {
		t.Fatalf("second=%#v", got)
	}
}

func TestBatchResumeAndEventIDsAreDeterministic(t *testing.T) {
	root := t.TempDir()
	writeAudit(t, root,
		core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), RequestID: "req-1", Type: "requested", Capability: "git", Action: "push"},
		core.CapabilityAuditEvent{Time: time.Unix(2, 0).UTC(), RequestID: "req-1", Type: "policy-decision", Capability: "git", Action: "push", Decision: core.PolicyRequireApproval},
		core.CapabilityAuditEvent{Time: time.Unix(3, 0).UTC(), RequestID: "req-1", Type: "approval-decision", Capability: "git", Action: "push", Approved: boolp(true)},
		core.CapabilityAuditEvent{Time: time.Unix(4, 0).UTC(), RequestID: "req-1", Type: "completed", Capability: "git", Action: "push", Success: boolp(true)},
	)
	reader, _ := NewReader(root)
	first, err := reader.Batch(context.Background(), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 2 {
		t.Fatalf("first=%#v", first)
	}
	if first.Events[0].EventID != "req-1:approval-required" || first.Events[1].EventID != "req-1:approval-approved" {
		t.Fatalf("ids=%#v", first.Events)
	}
	second, err := reader.Batch(context.Background(), first.NextOffset, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Events) != 1 || second.Events[0].EventID != "req-1:operation-completed" {
		t.Fatalf("second=%#v", second)
	}
	again, err := reader.Batch(context.Background(), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again.Events[0].EventID != first.Events[0].EventID || again.Events[1].EventID != first.Events[1].EventID {
		t.Fatalf("event ids changed: %#v %#v", first, again)
	}
}

func TestSkippedAuditRecordsStillAdvanceCursor(t *testing.T) {
	root := t.TempDir()
	path := writeAudit(t, root,
		core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), RequestID: "req-1", Type: "requested", Capability: "git", Action: "push"},
		core.CapabilityAuditEvent{Time: time.Unix(2, 0).UTC(), RequestID: "req-1", Type: "policy-decision", Capability: "git", Action: "push", Decision: core.PolicyAllow},
	)
	reader, _ := NewReader(root)
	batch, err := reader.Batch(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if len(batch.Events) != 0 || batch.NextOffset != info.Size() {
		t.Fatalf("batch=%#v size=%d", batch, info.Size())
	}
}

func TestTwoReadersObserveSameEventWithoutExecutingAnything(t *testing.T) {
	root := t.TempDir()
	writeAudit(t, root, core.CapabilityAuditEvent{Time: time.Unix(1, 0).UTC(), RequestID: "req-1", Type: "policy-decision", Capability: "git", Action: "push", Decision: core.PolicyRequireApproval})
	a, _ := NewReader(root)
	b, _ := NewReader(root)
	ba, err := a.Batch(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := b.Batch(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ba.Events) != 1 || len(bb.Events) != 1 || ba.Events[0].EventID != bb.Events[0].EventID {
		t.Fatalf("a=%#v b=%#v", ba, bb)
	}
}

func TestPublicCorruptionErrorDoesNotExposeInternalType(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit", "capabilities.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, _ := NewReader(root)
	batch, err := reader.Batch(context.Background(), 0, 10)
	if len(batch.Events) != 0 {
		t.Fatalf("batch=%#v", batch)
	}
	var corruption *CorruptionError
	if !errors.As(err, &corruption) {
		t.Fatalf("err=%T %v, want public CorruptionError", err, err)
	}
	if corruption.Line != 1 || corruption.ByteOffset != 0 || corruption.Kind != CorruptionMalformedJSON {
		t.Fatalf("corruption=%#v", corruption)
	}
}
