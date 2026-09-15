package control

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
)

// ProcessConn adapts a negotiated process session to the terminal bridge. Read
// drains output/credit frames; Write obeys the receiver's bounded input window.
// Result becomes available only after the final receipt AND session completion.
type ProcessConn struct {
	net.Conn
	frames       *processWriter
	stderr       io.Writer
	readMu       sync.Mutex
	output       []byte
	result       []byte
	complete     bool
	writeMu      sync.Mutex
	inputEOF     bool
	creditMu     sync.Mutex
	credit       int
	wake         chan struct{}
	closed       chan struct{}
	closeOnce    sync.Once
	wait         func() error
	inputStopped chan struct{}
	stopSeen     bool
}

func NewProcessConn(ctx context.Context, conn net.Conn, stderr io.Writer) (*ProcessConn, error) {
	if ctx == nil || conn == nil || stderr == nil {
		return nil, ErrInvalidArgument
	}
	session, ok := conn.(interface{ Wait(context.Context) error })
	if !ok {
		return nil, ErrInvalidArgument
	}
	c := &ProcessConn{Conn: conn, stderr: stderr, wake: make(chan struct{}, 1), closed: make(chan struct{}), inputStopped: make(chan struct{})}
	c.wait = func() error { return session.Wait(ctx) }
	c.frames = &processWriter{conn: conn, fail: func(error) { _ = c.Close() }}
	return c, nil
}

func (c *ProcessConn) Close() error {
	var err error
	c.closeOnce.Do(func() { close(c.closed); err = c.Conn.Close() })
	return err
}

func (c *ProcessConn) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.inputEOF {
		return 0, net.ErrClosed
	}
	written := 0
	for len(data) > 0 {
		select {
		case <-c.closed:
			return written, net.ErrClosed
		case <-c.inputStopped:
			return written, net.ErrClosed
		default:
		}
		c.creditMu.Lock()
		n := min(len(data), c.credit, processChunk)
		c.credit -= n
		c.creditMu.Unlock()
		if n == 0 {
			select {
			case <-c.closed:
				return written, net.ErrClosed
			case <-c.inputStopped:
				return written, net.ErrClosed
			case <-c.wake:
			}
			continue
		}
		if err := c.frames.frame(processInput, data[:n]); err != nil {
			// Read and Result independently establish completion. A peer that
			// exits before consuming stdin must not lose its actual exit status
			// to the terminal bridge's input-copy teardown error.
			return written, net.ErrClosed
		}
		written += n
		data = data[n:]
	}
	return written, nil
}

func (c *ProcessConn) CloseWrite() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.inputEOF {
		return nil
	}
	c.inputEOF = true
	if err := c.frames.frame(processInputEOF, nil); err != nil {
		return net.ErrClosed
	}
	return nil
}

func (c *ProcessConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	for len(c.output) == 0 {
		if c.complete {
			return 0, io.EOF
		}
		kind, data, err := readProcessFrame(c.Conn)
		if err != nil {
			_ = c.Close()
			if errors.Is(err, io.EOF) && len(c.result) != 0 {
				if err := c.wait(); err != nil {
					return 0, err
				}
				c.complete = true
				return 0, io.EOF
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return 0, ErrProtocol
			}
			return 0, err
		}
		if len(c.result) != 0 {
			_ = c.Close()
			return 0, ErrProtocol
		}
		if c.stopSeen && kind != processResult {
			_ = c.Close()
			return 0, ErrProtocol
		}
		switch kind {
		case processInputStop:
			if len(data) != 0 {
				_ = c.Close()
				return 0, ErrProtocol
			}
			c.stopSeen = true
			close(c.inputStopped)
			if err := c.CloseWrite(); err != nil {
				return 0, err
			}
		case processCredit:
			if len(data) != 4 {
				_ = c.Close()
				return 0, ErrProtocol
			}
			credit := binary.BigEndian.Uint32(data)
			c.creditMu.Lock()
			valid := credit > 0 && credit <= processChunk && c.credit+int(credit) <= processChunk
			if valid {
				c.credit += int(credit)
			}
			c.creditMu.Unlock()
			if !valid {
				_ = c.Close()
				return 0, ErrProtocol
			}
			select {
			case c.wake <- struct{}{}:
			default:
			}
		case processOutput:
			if len(data) == 0 {
				_ = c.Close()
				return 0, ErrProtocol
			}
			c.output = data
		case processErrorOutput:
			if len(data) == 0 {
				_ = c.Close()
				return 0, ErrProtocol
			}
			if err := writeProcessBytes(c.stderr, data); err != nil {
				_ = c.Close()
				return 0, err
			}
		case processResult:
			if len(data) == 0 || !c.stopSeen {
				_ = c.Close()
				return 0, ErrProtocol
			}
			c.result = data
		default:
			_ = c.Close()
			return 0, ErrProtocol
		}
	}
	n := copy(p, c.output)
	c.output = c.output[n:]
	return n, nil
}

func (c *ProcessConn) Result() ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if !c.complete {
		return nil, ErrProtocol
	}
	return append([]byte(nil), c.result...), nil
}

func (c *ProcessConn) SupportsResize() bool {
	r, ok := c.Conn.(interface{ SupportsResize() bool })
	return ok && r.SupportsResize()
}

func (c *ProcessConn) Resize(ctx context.Context, columns, rows int) error {
	r, ok := c.Conn.(interface {
		Resize(context.Context, int, int) error
	})
	if !ok {
		return ErrUnavailable
	}
	return r.Resize(ctx, columns, rows)
}
