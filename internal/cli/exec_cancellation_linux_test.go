//go:build linux

package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Cancellation publishes a parent's Done before propagation to every child has
// finished. The transport's AfterFunc may therefore close its connection before
// the terminal bridge's signal.NotifyContext child observes cancellation. This
// context controls that legal ordering without sleeps or scheduler probabilities.
// All callbacks are released or stopped and joined during cleanup.
type execStagedCancelContext struct {
	mu        sync.Mutex
	done      chan struct{}
	err       error
	callbacks []*execCancelCallback
}
type execCancelCallback struct {
	once sync.Once
	done chan struct{}
	run  func()
}

func (c *execCancelCallback) start() {
	c.once.Do(func() { go func() { defer close(c.done); c.run() }() })
}
func (c *execCancelCallback) stop() bool {
	stopped := false
	c.once.Do(func() { stopped = true; close(c.done) })
	return stopped
}
func (c *execStagedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *execStagedCancelContext) Done() <-chan struct{}       { return c.done }
func (c *execStagedCancelContext) Err() error                  { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *execStagedCancelContext) Value(any) any               { return nil }
func (c *execStagedCancelContext) AfterFunc(run func()) func() bool {
	callback := &execCancelCallback{run: run, done: make(chan struct{})}
	c.mu.Lock()
	c.callbacks = append(c.callbacks, callback)
	alreadyCanceled := c.err != nil
	c.mu.Unlock()
	if alreadyCanceled {
		callback.start()
	}
	return callback.stop
}
func (c *execStagedCancelContext) cancelTransportFirst(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	count := len(c.callbacks)
	if count != 2 {
		c.mu.Unlock()
		t.Fatalf("want transport and terminal-child callbacks, got %d", count)
	}
	c.err = context.Canceled
	close(c.done)
	transport := c.callbacks[0]
	c.mu.Unlock()
	transport.start()
}
func (c *execStagedCancelContext) finish(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	if c.err == nil {
		c.err = context.Canceled
		close(c.done)
	}
	callbacks := append([]*execCancelCallback(nil), c.callbacks...)
	c.mu.Unlock()
	for _, callback := range callbacks {
		callback.start()
	}
	for _, callback := range callbacks {
		execCancelAwait(t, callback.done, "context callback")
	}
}

// net.Pipe can choose EOF if both endpoints close together. Native network
// connections also expose the local-close outcome as net.ErrClosed. Select that
// documented outcome explicitly; the process API, framing, and bridge stay real.
type execCancelConn struct {
	net.Conn
	nativeClose bool
	localClosed atomic.Bool
	closeOnce   sync.Once
	closed      chan struct{}
}

func (c *execCancelConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil && c.nativeClose && c.localClosed.Load() {
		err = net.ErrClosed
	}
	return n, err
}
func (c *execCancelConn) Close() error {
	c.closeOnce.Do(func() { c.localClosed.Store(true); _ = c.Conn.Close(); close(c.closed) })
	return nil
}

type execCancelListener struct {
	connections chan net.Conn
	done        chan struct{}
	once        sync.Once
}

func (l *execCancelListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.connections:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}
func (l *execCancelListener) Close() error { l.once.Do(func() { close(l.done) }); return nil }
func (l *execCancelListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "exec-cancel-memory", Net: "unix"}
}

type execCancelService struct{ done chan struct{} }

func (s execCancelService) ExecStream(ctx context.Context, _ string, _ core.ProcessRequest, _ io.Reader, out, _ io.Writer) (core.ExecutionResult, error) {
	defer close(s.done)
	if _, err := io.WriteString(out, "started"); err != nil {
		return core.ExecutionResult{}, err
	}
	<-ctx.Done()
	return core.ExecutionResult{}, ctx.Err()
}

type execCancelOutput struct {
	ready chan struct{}
	once  sync.Once
}

func (w *execCancelOutput) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.ready) })
	return len(p), nil
}
func execCancelAwait(t *testing.T, ch <-chan struct{}, stage string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Errorf("timed out waiting for %s", stage)
	}
}

func TestExecCancellationPrecedesRealAPITransportCompletion(t *testing.T) {
	for _, nativeClose := range []bool{false, true} {
		name := "pipe-control"
		if nativeClose {
			name = "native-local-close"
		}
		t.Run(name, func(t *testing.T) {
			ctx := &execStagedCancelContext{done: make(chan struct{})}
			serverCtx, stopServer := context.WithCancel(context.Background())
			server := control.NewServer()
			operationDone := make(chan struct{})
			if err := controlapi.RegisterExec(server, execCancelService{done: operationDone}); err != nil {
				t.Fatal(err)
			}
			listener := &execCancelListener{connections: make(chan net.Conn), done: make(chan struct{})}
			serverDone := make(chan struct{})
			go func() { defer close(serverDone); _ = server.Serve(serverCtx, listener) }()
			local, remote := net.Pipe()
			clientConn := &execCancelConn{Conn: local, nativeClose: nativeClose, closed: make(chan struct{})}
			serverConn := &execCancelConn{Conn: remote, closed: make(chan struct{})}
			client, err := controlapi.NewClientWithDialer(func(callCtx context.Context) (net.Conn, error) {
				if err := callCtx.Err(); err != nil {
					return nil, err
				}
				select {
				case listener.connections <- serverConn:
					return clientConn, nil
				case <-callCtx.Done():
					return nil, callCtx.Err()
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			output := &execCancelOutput{ready: make(chan struct{})}
			commandDone := make(chan struct{})
			var code int
			t.Cleanup(func() {
				ctx.finish(t)
				_ = clientConn.Close()
				_ = serverConn.Close()
				stopServer()
				_ = listener.Close()
				execCancelAwait(t, commandDone, "CLI completion")
				execCancelAwait(t, operationDone, "server operation cleanup")
				execCancelAwait(t, clientConn.closed, "client connection closure")
				execCancelAwait(t, serverConn.closed, "server connection closure")
				execCancelAwait(t, serverDone, "server listener shutdown")
			})
			go func() {
				defer close(commandDone)
				code = execCommand(ctx, client, []string{"dev", "--", "sleep", "600"}, strings.NewReader(""), output, io.Discard)
			}()
			select {
			case <-output.ready:
			case <-time.After(5 * time.Second):
				t.Fatal("process did not stream its started marker")
			}
			ctx.cancelTransportFirst(t)
			select {
			case <-commandDone:
			case <-time.After(5 * time.Second):
				t.Fatal("exec did not return after transport cancellation")
			}
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("caller cancellation was lost: %v", ctx.Err())
			}
			if code != 130 {
				t.Fatalf("canceled exec returned %d, want 130", code)
			}
		})
	}
}
