package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

const methodSessionResize = "_control.session.resize"

type terminalControlKey struct{}

// terminalControl is private to one stream. Its callback is installed during
// preparation, before the session identity or resize capability is published.
type terminalControl struct {
	mu     sync.Mutex
	resize func(int, int) error
	closed bool
}

// SetTerminalResizeHandler enables size control for this prepared stream only.
// The transport serializes callbacks and rejects controls after completion.
func SetTerminalResizeHandler(ctx context.Context, resize func(int, int) error) {
	state, _ := ctx.Value(terminalControlKey{}).(*terminalControl)
	if state != nil {
		state.resize = resize
	}
}

type sessionResizeRequest struct {
	SessionID string `json:"session_id"`
	Columns   int    `json:"columns"`
	Rows      int    `json:"rows"`
}

func (s *Server) resizeSession(payload json.RawMessage) error {
	var request sessionResizeRequest
	if json.Unmarshal(payload, &request) != nil || len(request.SessionID) != 2*sessionIDBytes ||
		request.Columns < 1 || request.Columns > 10000 || request.Rows < 1 || request.Rows > 10000 {
		return NewStatusError("invalid_argument", "invalid terminal resize request")
	}
	s.sessionMu.Lock()
	state := s.sessions[request.SessionID]
	s.sessionMu.Unlock()
	if state == nil || state.terminal == nil {
		return NewStatusError("not_found", "terminal session not found")
	}
	terminal := state.terminal
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if terminal.closed || terminal.resize == nil {
		return NewStatusError("unsupported", "session does not support terminal resize")
	}
	return terminal.resize(request.Columns, request.Rows)
}

// SupportsResize reports a capability negotiated in the stream handshake.
func (c *sessionConn) SupportsResize() bool { return c.resize }

// Resize uses a separate bounded RPC. Control data never enters process stdin.
func (c *sessionConn) Resize(ctx context.Context, columns, rows int) error {
	if !c.resize {
		return NewStatusError("unsupported", "peer does not support terminal resize")
	}
	err := c.client.Call(ctx, methodSessionResize, sessionResizeRequest{c.id, columns, rows}, nil)
	var status *StatusError
	if errors.As(err, &status) && (status.Code == "not_found" || status.Code == "unsupported") {
		// A final window event may race normal process completion. The stream
		// still owns final output and the actual exit result; do not close it.
		return io.EOF
	}
	return err
}
