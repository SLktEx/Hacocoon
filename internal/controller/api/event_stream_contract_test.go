package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func TestEventStreamKeepsSafeCursor(t *testing.T) {
	for _, mode := range []string{"mismatch", "backwards-event", "duplicate-event", "callback-failure", "backwards-terminal", "backwards-error", "mixed-error", "negative", "event-and-done", "empty", "eof", "invalid-json", "success"} {
		t.Run(mode, func(t *testing.T) {
			frames := []eventStreamFrame{{Event: &eventsapp.Event{Type: "requested", NextOffset: 10}, NextOffset: 10}}
			second := eventStreamFrame{Event: &eventsapp.Event{Type: "completed", NextOffset: 20}, NextOffset: 20}
			switch mode {
			case "mismatch":
				second.NextOffset = 999
			case "backwards-event":
				second.NextOffset = 5
				second.Event.NextOffset = 5
			case "duplicate-event":
				second.NextOffset = 10
				second.Event.NextOffset = 10
			case "backwards-terminal":
				second = eventStreamFrame{Done: true, NextOffset: 5}
			case "backwards-error":
				second = eventStreamFrame{Error: &responseStatus{Code: "internal", Message: "source failed"}, NextOffset: 5}
			case "mixed-error":
				second.Error = &responseStatus{Code: "internal", Message: "source failed"}
			case "negative":
				second.NextOffset = -1
			case "event-and-done":
				second.Done = true
			case "empty":
				second = eventStreamFrame{}
			}
			if mode != "eof" && mode != "invalid-json" {
				frames = append(frames, second)
			}
			if mode == "success" {
				frames = append(frames, eventStreamFrame{Done: true, NextOffset: 20})
			}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := s.RegisterStream(MethodEventsStream, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, conn net.Conn) error {
						for _, frame := range frames {
							if err := json.NewEncoder(conn).Encode(frame); err != nil {
								return err
							}
						}
						if mode == "invalid-json" {
							_, err := io.WriteString(conn, "{broken}\n")
							return err
						}
						return nil
					}, nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			consumerError := errors.New("consumer failed before committing event")
			var received []string
			next, err := client.StreamEvents(context.Background(), 0, func(event eventsapp.Event) error {
				if event.Type == "completed" && mode == "callback-failure" {
					return consumerError
				}
				received = append(received, event.Type)
				return nil
			})
			wantNext := int64(10)
			wantEvents := []string{"requested"}
			if mode == "success" {
				wantNext = 20
				wantEvents = append(wantEvents, "completed")
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("invalid/unfinished stream reported success")
			}
			if mode == "callback-failure" && !errors.Is(err, consumerError) {
				t.Fatal("consumer error lost", err)
			}
			if next != wantNext || !reflect.DeepEqual(received, wantEvents) {
				t.Fatal("invalid or uncommitted event changed resume boundary", next, received, err)
			}
		})
	}
}
