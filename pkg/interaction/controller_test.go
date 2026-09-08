package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func TestControllerReaderProjectsResumesAndBoundsBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	err = server.RegisterStream(controlapi.MethodEventsStream, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		var request controlapi.EventsStreamRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, err
		}
		return func(ctx context.Context, conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			for _, offset := range []int64{10, 20, 30} {
				if offset <= request.SinceOffset {
					continue
				}
				event := eventsapp.Event{Type: "policy-decision", Decision: core.PolicyRequireApproval, RequestID: "request", Capability: "demo", NextOffset: offset, Resource: "SECRET_RESOURCE", Attributes: map[string]string{"credential": "SECRET_ATTRIBUTE"}, Reason: "SECRET_REASON"}
				if offset == 10 {
					event.Type = "requested"
				}
				if err := encoder.Encode(map[string]any{"event": event, "next_offset": offset}); err != nil {
					return err
				}
			}
			return encoder.Encode(map[string]any{"done": true, "next_offset": int64(30)})
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server failed to stop")
		}
	})
	t.Setenv("HACO_CLIENT_MODE", "controller")
	t.Setenv("HACO_CONTROL_SOCKET", path)
	t.Setenv("HACO_ROOT", filepath.Join(t.TempDir(), "absent-audit"))
	reader, err := NewDefaultReader()
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reader.Batch(ctx, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.NextOffset != 20 {
		t.Fatalf("first batch=%+v", batch)
	}
	payload, _ := json.Marshal(batch)
	if strings.Contains(string(payload), "SECRET") {
		t.Fatalf("private event leaked: %s", payload)
	}
	batch, err = reader.Batch(ctx, batch.NextOffset, 10)
	if err != nil || len(batch.Events) != 1 || batch.NextOffset != 30 {
		t.Fatalf("resume=%+v err=%v", batch, err)
	}
	batch, err = reader.Batch(ctx, batch.NextOffset, 10)
	if err != nil || len(batch.Events) != 0 || batch.NextOffset != 30 {
		t.Fatalf("tail=%+v err=%v", batch, err)
	}
	sentinel := errors.New("consumer stopped")
	_, err = reader.Stream(ctx, 0, func(Event) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("consumer error lost: %v", err)
	}
}

func TestControllerReaderDoesNotFallBackToLocalAudit(t *testing.T) {
	t.Setenv("HACO_ROOT", t.TempDir())
	t.Setenv("HACO_CLIENT_MODE", "controller")
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "SECRET_SOCKET"))
	reader, err := NewDefaultReader()
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reader.Batch(context.Background(), 0, 1)
	if err == nil || len(batch.Events) != 0 || batch.NextOffset != 0 || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	t.Setenv("HACO_CLIENT_MODE", "unknown")
	if _, err := NewDefaultReader(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("mode error=%v", err)
	}
}
