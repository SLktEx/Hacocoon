package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
)

func TestSetupProgressRequiresOneConsistentCompletedOperation(t *testing.T) {
	id := strings.Repeat("a", 32)
	event := func(stage, state string) setupFrame {
		return setupFrame{RequestID: id, Event: &hostsetup.Event{Stage: stage, State: state}}
	}
	start := event("setup", "running")
	complete := event("setup", "succeeded")
	result := setupFrame{RequestID: id, Result: &recipes.HostResult{State: "none"}}
	done := setupFrame{Done: true}
	for _, mode := range []string{"result-only", "ordinary", "result-missing", "result-before-start", "result-twice", "result-wrong-operation", "result-invalid", "result-with-event", "done-with-result", "done-with-id", "unknown-status", "wrong-operation", "invalid-hex", "negative-duration", "unstarted-stage", "unfinished-stage", "second-start", "event-after-complete", "reason-on-success", "missing-failure-reason", "unknown-state", "frame-limit", "failed-result-only", "failed-result-as-success"} {
		t.Run(mode, func(t *testing.T) {
			frames := []setupFrame{start, result, complete, done}
			update := recipes.Update{ResultOnly: true}
			wantEvents, wantResults := 1, 1
			valid := mode == "result-only" || mode == "ordinary" || mode == "failed-result-only"
			switch mode {
			case "ordinary":
				update.ResultOnly = false
				frames = []setupFrame{start, complete, done}
				wantResults = 0
			case "result-missing":
				frames = []setupFrame{start, complete, done}
				wantEvents, wantResults = 2, 0
			case "result-before-start":
				frames = []setupFrame{result, done}
				wantEvents, wantResults = 0, 0
			case "result-twice":
				frames = []setupFrame{start, result, result, complete, done}
			case "result-wrong-operation":
				frames[1].RequestID = strings.Repeat("b", 32)
				wantResults = 0
			case "result-invalid":
				frames[1].Result = &recipes.HostResult{State: "succeeded"}
				wantResults = 0
			case "result-with-event":
				frames[1].Event = complete.Event
				wantResults = 0
			case "done-with-result":
				frames[3].Result = result.Result
				wantEvents = 2
			case "done-with-id":
				frames[3].RequestID = id
				wantEvents = 2
			case "unknown-status":
				frames[3].Code = "provider-secret"
				wantEvents = 2
			case "wrong-operation":
				frames[2].RequestID = strings.Repeat("b", 32)
			case "invalid-hex":
				frames[0].RequestID = strings.Repeat("z", 32)
				wantEvents, wantResults = 0, 0
			case "negative-duration":
				f := event("setup", "succeeded")
				f.Event.DurationMS = -1
				frames[2] = f
			case "unstarted-stage":
				frames[2] = event("wsl_interop", "succeeded")
			case "unfinished-stage":
				frames = []setupFrame{start, event("wsl_interop", "running"), complete, done}
				wantEvents, wantResults = 2, 0
			case "second-start":
				frames[2] = start
			case "event-after-complete":
				frames = []setupFrame{start, result, complete, event("wsl_interop", "running"), done}
				wantEvents = 2
			case "reason-on-success":
				f := event("setup", "succeeded")
				f.Event.Reason = "failed"
				frames[2] = f
			case "missing-failure-reason":
				frames[2] = event("setup", "failed")
			case "unknown-state":
				frames[2] = event("setup", "interrupted")
			case "frame-limit":
				frames = []setupFrame{start}
				for range 260 {
					frames = append(frames, event("wsl_interop", "running"), event("wsl_interop", "succeeded"))
				}
				frames = append(frames, result, complete, done)
				wantEvents, wantResults = 512, 0
			case "failed-result-only", "failed-result-as-success":
				frames[1].Result = &recipes.HostResult{State: "failed", Instance: "11111111-1111-4111-8111-111111111111", Digest: strings.Repeat("a", 64)}
				update.ResultOnly = mode == "failed-result-only"
				wantEvents = 2
			}
			if valid {
				wantEvents = 2
			}
			var requests atomic.Int32
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := s.RegisterStream(MethodSetupProgress, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
					requests.Add(1)
					var received recipes.Update
					if err := json.Unmarshal(payload, &received); err != nil || !reflect.DeepEqual(received, update) {
						return nil, errors.New("setup intent changed")
					}
					return func(_ context.Context, conn net.Conn) error {
						for _, f := range frames {
							if err := json.NewEncoder(conn).Encode(f); err != nil {
								return err
							}
						}
						return nil
					}, nil
				}); err != nil {
					t.Fatal(err)
				}
				if err := s.Register(MethodSetup, func(context.Context, json.RawMessage) (any, error) {
					requests.Add(1)
					return PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			seen, results := 0, 0
			err = client.SetupHostProgress(context.Background(), update, func(requestID string, _ hostsetup.Event) {
				if requestID != id {
					t.Error("unverified operation reached UI", requestID)
				}
				seen++
			}, nil, func(value recipes.HostResult) {
				if !value.Valid() || !reflect.DeepEqual(value, *frames[1].Result) {
					t.Error("unverified customization result reached UI", value)
				}
				results++
			})
			if (valid && err != nil) || (!valid && !errors.Is(err, control.ErrProtocol)) {
				t.Fatal("setup completion contract changed", err)
			}
			if seen != wantEvents || results != wantResults || requests.Load() != 1 {
				t.Fatal("invalid frames were published or setup was repeated", seen, results, requests.Load())
			}
		})
	}
}
