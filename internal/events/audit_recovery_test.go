package events

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestAuditStreamRejectsCompleteJSONWithoutNewlineAndResumesAfterAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	line := `{"time":"2026-09-13T00:00:00Z","type":"requested","environment_instance":"env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	service := New(path)
	if _, err := service.StreamAudit(context.Background(), int64(len(line)), func(core.CapabilityAuditEvent, int64) error { return nil }); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("unterminated EOF offset accepted: %v", err)
	}
	calls := 0
	emit := func(event core.CapabilityAuditEvent, _ int64) error {
		calls++
		if event.EnvironmentInstance == "" {
			t.Fatal("identity lost")
		}
		return nil
	}
	offset, err := service.StreamAudit(context.Background(), 0, emit)
	var corrupt *AuditCorruptionError
	if !errors.As(err, &corrupt) || corrupt.Kind != CorruptionIncomplete || offset != 0 || calls != 0 {
		t.Fatalf("offset=%d calls=%d err=%v", offset, calls, err)
	}
	if err := os.WriteFile(path, []byte(line+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	offset, err = service.StreamAudit(context.Background(), offset, emit)
	if err != nil || calls != 1 || offset != int64(len(line)+1) {
		t.Fatalf("%d %d %v", offset, calls, err)
	}
}

func TestAuditStreamHasFixedReadBoundaryDuringAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	line := []byte("{\"time\":\"2026-09-13T00:00:00Z\",\"type\":\"requested\"}\n")
	if err := os.WriteFile(path, line, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	offset, err := New(path).StreamAudit(context.Background(), 0, func(core.CapabilityAuditEvent, int64) error {
		calls++
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(line)
		return err
	})
	if err != nil || calls != 1 || offset != int64(len(line)) {
		t.Fatalf("%d %d %v", offset, calls, err)
	}
}
