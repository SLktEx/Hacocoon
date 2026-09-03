package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/terminalsession"
)

const (
	methodSessionWait   = "_control.session.wait"
	methodSessionResize = "_control.session.resize"
	sessionIDBytes      = 16
	sessionRetention    = 2 * time.Minute
	maxSessionDimension = 10000
)

type sessionWaitRequest struct {
	SessionID string `json:"session_id"`
}

type sessionWaitResponse struct {
	ExitCode int `json:"exit_code"`
}

type sessionResizeRequest struct {
	SessionID string `json:"session_id"`
	Columns   int    `json:"columns"`
	Rows      int    `json:"rows"`
}

// SessionExitError represents a successfully established process session that
// completed with a non-zero process exit status.
type SessionExitError struct {
	Code int
}

func (e *SessionExitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("remote session exited %d", e.Code)
}

func (e *SessionExitError) ExitCode() int {
	if e == nil {
		return 0
	}
	return e.Code
}

type serverSession struct {
	done   chan struct{}
	resize chan terminalsession.Size
	once   sync.Once
	err    error
}

func newServerSession() *serverSession {
	return &serverSession{
		done:   make(chan struct{}),
		resize: make(chan terminalsession.Size, 1),
	}
}

func (s *serverSession) ResizeEvents() <-chan terminalsession.Size {
	if s == nil {
		return nil
	}
	return s.resize
}

func (s *serverSession) complete(err error) {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.err = err
		close(s.done)
	})
}

func (s *serverSession) pushResize(size terminalsession.Size) error {
	if s == nil {
		return ErrInvalidArgument
	}
	select {
	case <-s.done:
		return NewStatusError("not_found", "session is already complete")
	default:
	}

	// Window drags can generate resize events faster than the runtime needs to
	// apply them. Keep only the newest pending geometry instead of allowing an
	// unbounded session-control queue to grow.
	select {
	case s.resize <- size:
		return nil
	default:
	}
	select {
	case <-s.resize:
	default:
	}
	select {
	case s.resize <- size:
	default:
	}
	return nil
}

func newSessionID() (string, error) {
	buffer := make([]byte, sessionIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate control session id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func (s *Server) createSession() (string, *serverSession, error) {
	if s == nil {
		return "", nil, ErrInvalidArgument
	}
	for attempts := 0; attempts < 4; attempts++ {
		id, err := newSessionID()
		if err != nil {
			return "", nil, err
		}
		state := newServerSession()
		s.sessionMu.Lock()
		if _, exists := s.sessions[id]; !exists {
			s.sessions[id] = state
			s.sessionMu.Unlock()
			return id, state, nil
		}
		s.sessionMu.Unlock()
	}
	return "", nil, fmt.Errorf("allocate unique control session id: %w", ErrUnavailable)
}

func (s *Server) discardSession(id string, state *serverSession) {
	if s == nil || id == "" || state == nil {
		return
	}
	s.sessionMu.Lock()
	if s.sessions[id] == state {
		delete(s.sessions, id)
	}
	s.sessionMu.Unlock()
}

func (s *Server) completeSession(id string, state *serverSession, err error) {
	if s == nil || id == "" || state == nil {
		return
	}
	state.complete(err)
	time.AfterFunc(sessionRetention, func() {
		s.discardSession(id, state)
	})
}

func (s *Server) session(id string) *serverSession {
	if s == nil || id == "" {
		return nil
	}
	s.sessionMu.Lock()
	state := s.sessions[id]
	s.sessionMu.Unlock()
	return state
}

func (s *Server) waitSession(ctx context.Context, id string) (sessionWaitResponse, error) {
	if s == nil || id == "" {
		return sessionWaitResponse{}, NewStatusError("invalid_argument", "session_id is required")
	}
	state := s.session(id)
	if state == nil {
		return sessionWaitResponse{}, NewStatusError("not_found", "session not found")
	}
	select {
	case <-state.done:
	case <-ctx.Done():
		return sessionWaitResponse{}, ctx.Err()
	}

	s.discardSession(id, state)
	if state.err == nil {
		return sessionWaitResponse{ExitCode: 0}, nil
	}
	var exitCoder interface{ ExitCode() int }
	if errors.As(state.err, &exitCoder) {
		if code := exitCoder.ExitCode(); code >= 0 {
			return sessionWaitResponse{ExitCode: code}, nil
		}
	}
	return sessionWaitResponse{}, state.err
}

func (s *Server) resizeSession(request sessionResizeRequest) error {
	if s == nil || request.SessionID == "" || request.Columns <= 0 || request.Rows <= 0 || request.Columns > maxSessionDimension || request.Rows > maxSessionDimension {
		return NewStatusError("invalid_argument", "valid session_id, columns, and rows are required")
	}
	state := s.session(request.SessionID)
	if state == nil {
		return NewStatusError("not_found", "session not found")
	}
	return state.pushResize(terminalsession.Size{Columns: request.Columns, Rows: request.Rows})
}

type sessionConn struct {
	net.Conn
	client *Client
	id     string
	ctx    context.Context

	waitOnce sync.Once
	waitErr  error
}

func (c *sessionConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if !errors.Is(err, io.EOF) {
		return n, err
	}
	if waitErr := c.Wait(c.ctx); waitErr != nil {
		return n, waitErr
	}
	return n, err
}

// Wait waits for the server-side stream/process completion result. The result is
// cached so repeated EOF reads cannot consume the one-shot completion twice.
func (c *sessionConn) Wait(ctx context.Context) error {
	if c == nil || c.client == nil || c.id == "" {
		return ErrInvalidArgument
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.waitOnce.Do(func() {
		var response sessionWaitResponse
		if err := c.client.Call(ctx, methodSessionWait, sessionWaitRequest{SessionID: c.id}, &response); err != nil {
			c.waitErr = err
			return
		}
		if response.ExitCode != 0 {
			c.waitErr = &SessionExitError{Code: response.ExitCode}
		}
	})
	return c.waitErr
}

// ResizeTerminal sends terminal geometry over the independent session control
// connection. Raw PTY stdin/stdout bytes remain untouched.
func (c *sessionConn) ResizeTerminal(ctx context.Context, columns, rows int) error {
	if c == nil || c.client == nil || c.id == "" || columns <= 0 || rows <= 0 {
		return ErrInvalidArgument
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.client.Call(ctx, methodSessionResize, sessionResizeRequest{
		SessionID: c.id,
		Columns:   columns,
		Rows:      rows,
	}, nil)
}

func (c *sessionConn) CloseWrite() error {
	if closer, ok := c.Conn.(closeWriter); ok {
		return closer.CloseWrite()
	}
	return nil
}
