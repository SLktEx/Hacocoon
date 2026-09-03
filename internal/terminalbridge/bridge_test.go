package terminalbridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPrepareInteractiveTerminalLeavesNonTTYInputUntouched(t *testing.T) {
	restore, err := PrepareInteractiveTerminal(bytes.NewBufferString("exit\n"))
	if err != nil {
		t.Fatal(err)
	}
	if restore != nil {
		t.Fatal("non-TTY input unexpectedly returned a restore function")
	}
}

func TestBridgeWithTerminalPreparesRestoresAndCopies(t *testing.T) {
	conn := newScriptedConn("remote output\n")
	var stdout bytes.Buffer
	prepared := false
	restored := false

	err := BridgeWithTerminal(
		context.Background(),
		conn,
		strings.NewReader("local input\n"),
		&stdout,
		func(io.Reader) (func() error, error) {
			prepared = true
			return func() error {
				restored = true
				return nil
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared {
		t.Fatal("terminal was not prepared")
	}
	if !restored {
		t.Fatal("terminal was not restored")
	}
	if got := conn.input.String(); got != "local input\n" {
		t.Fatalf("input copied to stream = %q", got)
	}
	if got := stdout.String(); got != "remote output\n" {
		t.Fatalf("stream output = %q", got)
	}
	if !conn.isClosed() {
		t.Fatal("stream was not closed")
	}
}

func TestBridgeWithTerminalRestoresOnOutputFailure(t *testing.T) {
	conn := newFailingReadConn(errors.New("read failed"))
	restored := false

	err := BridgeWithTerminal(
		context.Background(),
		conn,
		strings.NewReader("input"),
		io.Discard,
		func(io.Reader) (func() error, error) {
			return func() error {
				restored = true
				return nil
			}, nil
		},
	)
	if err == nil || err.Error() != "read failed" {
		t.Fatalf("error = %v, want read failed", err)
	}
	if !restored {
		t.Fatal("terminal was not restored after stream failure")
	}
}

func TestBridgeWithTerminalRestoresOnContextCancellation(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	prepared := make(chan struct{})
	restored := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- BridgeWithTerminal(
			ctx,
			client,
			strings.NewReader(""),
			io.Discard,
			func(io.Reader) (func() error, error) {
				close(prepared)
				return func() error {
					close(restored)
					return nil
				}, nil
			},
		)
	}()

	select {
	case <-prepared:
	case <-time.After(time.Second):
		t.Fatal("terminal preparation did not start")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("bridge did not return after cancellation")
	}
	select {
	case <-restored:
	case <-time.After(time.Second):
		t.Fatal("terminal was not restored after cancellation")
	}
}

type testAddr string

func (a testAddr) Network() string { return "test" }
func (a testAddr) String() string  { return string(a) }

type scriptedConn struct {
	input         bytes.Buffer
	output        *strings.Reader
	writeClosed   chan struct{}
	closed        chan struct{}
	closeWriteOne sync.Once
	closeOne      sync.Once
}

func newScriptedConn(output string) *scriptedConn {
	return &scriptedConn{
		output:      strings.NewReader(output),
		writeClosed: make(chan struct{}),
		closed:      make(chan struct{}),
	}
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	select {
	case <-c.writeClosed:
		return c.output.Read(p)
	case <-c.closed:
		return 0, net.ErrClosed
	}
}

func (c *scriptedConn) Write(p []byte) (int, error) { return c.input.Write(p) }
func (c *scriptedConn) CloseWrite() error {
	c.closeWriteOne.Do(func() { close(c.writeClosed) })
	return nil
}
func (c *scriptedConn) Close() error {
	c.closeOne.Do(func() { close(c.closed) })
	return nil
}
func (c *scriptedConn) LocalAddr() net.Addr                 { return testAddr("local") }
func (c *scriptedConn) RemoteAddr() net.Addr                { return testAddr("remote") }
func (c *scriptedConn) SetDeadline(time.Time) error         { return nil }
func (c *scriptedConn) SetReadDeadline(time.Time) error     { return nil }
func (c *scriptedConn) SetWriteDeadline(time.Time) error    { return nil }
func (c *scriptedConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

type failingReadConn struct {
	err    error
	closed bool
}

func newFailingReadConn(err error) *failingReadConn { return &failingReadConn{err: err} }
func (c *failingReadConn) Read([]byte) (int, error)  { return 0, c.err }
func (c *failingReadConn) Write(p []byte) (int, error) {
	return len(p), nil
}
func (c *failingReadConn) Close() error {
	c.closed = true
	return nil
}
func (c *failingReadConn) CloseWrite() error              { return nil }
func (c *failingReadConn) LocalAddr() net.Addr            { return testAddr("local") }
func (c *failingReadConn) RemoteAddr() net.Addr           { return testAddr("remote") }
func (c *failingReadConn) SetDeadline(time.Time) error     { return nil }
func (c *failingReadConn) SetReadDeadline(time.Time) error { return nil }
func (c *failingReadConn) SetWriteDeadline(time.Time) error {
	return nil
}
