package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerRefusesInvalidRequestsBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, request, code string
	}{
		{"malformed", "[\n", "internal"},
		{"version", `{"version":999,"method":"operation"}`, "protocol_version"},
		{"missing-method", `{"version":1,"method":" "}`, "invalid_argument"},
		{"session-without-stream", `{"version":1,"method":"operation","session":true}`, "invalid_argument"},
		{"unknown-call", `{"version":1,"method":"unknown"}`, "not_found"},
		{"unknown-stream", `{"version":1,"method":"unknown","stream":true}`, "not_found"},
		{"resize-stream", `{"version":1,"method":"_control.session.resize","stream":true}`, "invalid_argument"},
		{"resize-shape", `{"version":1,"method":"_control.session.resize","payload":[]}`, "invalid_argument"},
		{"cancel-stream", `{"version":1,"method":"_control.session.cancel","stream":true}`, "invalid_argument"},
		{"cancel-shape", `{"version":1,"method":"_control.session.cancel","payload":[]}`, "invalid_argument"},
		{"cancel-missing-id", `{"version":1,"method":"_control.session.cancel","payload":{}}`, "invalid_argument"},
		{"cancel-unknown", `{"version":1,"method":"_control.session.cancel","payload":{"session_id":"0123456789abcdef0123456789abcdef"}}`, "not_found"},
		{"wait-stream", `{"version":1,"method":"_control.session.wait","stream":true}`, "invalid_argument"},
		{"wait-shape", `{"version":1,"method":"_control.session.wait","payload":[]}`, "invalid_argument"},
		{"wait-missing-id", `{"version":1,"method":"_control.session.wait","payload":{}}`, "invalid_argument"},
		{"wait-unknown", `{"version":1,"method":"_control.session.wait","payload":{"session_id":"unknown"}}`, "not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer()
			var calls atomic.Int32
			if err := server.Register("operation", func(context.Context, json.RawMessage) (any, error) {
				calls.Add(1)
				return nil, nil
			}); err != nil {
				t.Fatal(err)
			}
			response := serverBoundaryResponse(t, server, strings.TrimSuffix(tc.request, "\n")+"\n")
			if response.Version != ProtocolVersion || response.Error == nil || response.Error.Code != tc.code || len(response.Payload) != 0 || calls.Load() != 0 {
				t.Fatal("invalid request dispatched or acknowledged", response, response.Error, calls.Load())
			}
		})
	}
}

func serverBoundaryResponse(t *testing.T, server *Server, request string) responseEnvelope {
	t.Helper()
	client, wire := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = wire.Close() })
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); server.serveConn(context.Background(), wire) }()
	if _, err := io.WriteString(client, request); err != nil {
		t.Fatal(err)
	}
	var response responseEnvelope
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(client); err != nil || len(data) != 0 {
		t.Fatal("response included an unexpected stream", len(data), err)
	}
	<-done
	return response
}

func TestServerRejectsFailedPreparedOperations(t *testing.T) {
	for _, mode := range []string{"unencodable-result", "nil-stream", "refused-stream"} {
		t.Run(mode, func(t *testing.T) {
			server := NewServer()
			request := `{"version":1,"method":"operation"}`
			wantCode := "internal"
			var registerErr error
			if mode == "unencodable-result" {
				registerErr = server.Register("operation", func(context.Context, json.RawMessage) (any, error) { return make(chan int), nil })
			} else {
				request = `{"version":1,"method":"operation","stream":true,"session":true}`
				registerErr = server.RegisterStream("operation", func(context.Context, json.RawMessage) (Stream, error) {
					if mode == "refused-stream" {
						return nil, NewStatusError("policy_denied", "operation refused")
					}
					return nil, nil
				})
				if mode == "refused-stream" {
					wantCode = "policy_denied"
				}
			}
			if registerErr != nil {
				t.Fatal(registerErr)
			}
			response := serverBoundaryResponse(t, server, request+"\n")
			if response.Error == nil || response.Error.Code != wantCode || response.SessionID != "" || len(response.Payload) != 0 || len(server.sessions) != 0 {
				t.Fatal("failed preparation acknowledged a session or result", response)
			}
		})
	}
}

func TestServerLostAcknowledgementDiscardsUnstartedSession(t *testing.T) {
	server := NewServer()
	var started atomic.Bool
	if err := server.RegisterStream("operation", func(context.Context, json.RawMessage) (Stream, error) {
		return func(context.Context, net.Conn) error { started.Store(true); return nil }, nil
	}); err != nil {
		t.Fatal(err)
	}
	client, wire := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = wire.Close() })
	done := make(chan struct{})
	go func() { defer close(done); server.serveConn(context.Background(), wire) }()
	if _, err := io.WriteString(client, `{"version":1,"method":"operation","stream":true,"session":true}`+"\n"); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lost handshake retained connection")
	}
	if started.Load() || len(server.sessions) != 0 {
		t.Fatal("unacknowledged session was started or retained")
	}
}

func TestServerMethodRegistrationCannotReplaceAnOwner(t *testing.T) {
	call := func(context.Context, json.RawMessage) (any, error) { return "original", nil }
	stream := func(context.Context, json.RawMessage) (Stream, error) { return nil, nil }
	for _, firstStream := range []bool{false, true} {
		server := NewServer()
		var err error
		if firstStream {
			err = server.RegisterStream("owned", stream)
		} else {
			err = server.Register("owned", call)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, err := range []error{server.Register("owned", call), server.RegisterStream("owned", stream), server.Register("", call), server.RegisterStream(" ", stream), server.Register("missing", nil), server.RegisterStream("missing", nil), (*Server)(nil).Register("method", call), (*Server)(nil).RegisterStream("method", stream)} {
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatal("method owner replaced or invalid registration accepted", err)
			}
		}
		if len(server.handlers)+len(server.streamHandlers) != 1 {
			t.Fatal("failed registration changed dispatch table")
		}
	}
}
