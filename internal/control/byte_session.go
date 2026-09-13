package control

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

func (s *Server) cancelSession(ctx context.Context, id string) error {
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != sessionIDBytes {
		return ErrInvalidArgument
	}
	s.sessionMu.Lock()
	state := s.sessions[id]
	s.sessionMu.Unlock()
	if state == nil {
		return NewStatusError("not_found", "session not found")
	}
	if state.cancel == nil {
		return ErrUnavailable
	}
	state.cancel()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-state.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Half-close is data-plane EOF, while Close before both EOFs is an explicit
// cancellation. The independent management call can stop a target that ignores
// EOF without forbidding a valid response after client CloseWrite.
type byteSessionConn struct {
	*sessionConn
	readEOF   atomic.Bool
	writeEOF  atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func (c *byteSessionConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if errors.Is(err, io.EOF) {
		c.readEOF.Store(true)
	}
	return n, err
}
func (c *byteSessionConn) CloseWrite() error {
	err := c.sessionConn.CloseWrite()
	if err == nil {
		c.writeEOF.Store(true)
	}
	return err
}
func (c *byteSessionConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.Conn.Close()
		if !c.readEOF.Load() || !c.writeEOF.Load() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 6*time.Second)
			defer cancel()
			c.closeErr = errors.Join(c.closeErr, c.client.Call(ctx, methodSessionCancel, sessionWaitRequest{SessionID: c.id}, nil))
		}
	})
	return c.closeErr
}
