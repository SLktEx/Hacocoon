package streamio

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// A concurrent second Close may report ErrClosed before the original call has
// completed. Hold that first call after unblocking I/O to expose this ordering.
type pendingClose struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
	calls    atomic.Int32
}

func (p *pendingClose) close(closeSocket func() error) error {
	if p.calls.Add(1) != 1 {
		return net.ErrClosed
	}
	err := closeSocket()
	close(p.started)
	<-p.release
	close(p.finished)
	return err
}

type pendingCloseListener struct {
	net.Listener
	gate *pendingClose
}

func (l pendingCloseListener) Close() error { return l.gate.close(l.Listener.Close) }

type pendingCloseConn struct {
	net.Conn
	gate *pendingClose
}

func (c pendingCloseConn) Close() error { return c.gate.close(c.Conn.Close) }

type pendingAcceptedCloseListener struct {
	net.Listener
	gate *pendingClose
}

func (l pendingAcceptedCloseListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return pendingCloseConn{Conn: c, gate: l.gate}, nil
}

func TestServeWaitsForCancellationCloseCompletion(t *testing.T) {
	for _, target := range []string{"listener", "accepted-connection"} {
		t.Run(target, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = listener.Close() }()
			gate := &pendingClose{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
			var served net.Listener = pendingCloseListener{Listener: listener, gate: gate}
			if target == "accepted-connection" {
				served = pendingAcceptedCloseListener{Listener: listener, gate: gate}
			}
			opened := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- Serve(ctx, served, func(ctx context.Context) (net.Conn, error) {
					close(opened)
					<-ctx.Done()
					// Make the cancellation callback own the first Close call.
					<-gate.started
					return nil, ctx.Err()
				}, nil)
			}()
			if target == "accepted-connection" {
				conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
				if err != nil {
					close(gate.release)
					t.Fatal(err)
				}
				defer func() { _ = conn.Close() }()
				select {
				case <-opened:
				case <-time.After(time.Second):
					close(gate.release)
					t.Fatal("connection was not accepted")
				}
			}
			cancel()
			assertCloseCompletion(t, "Serve", gate, done)
		})
	}
}

func TestRelayWaitsForCancellationCloseCompletion(t *testing.T) {
	for _, target := range []string{"local", "upstream"} {
		t.Run(target, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			local, user := net.Pipe()
			upstream, application := net.Pipe()
			defer func() { _ = local.Close(); _ = user.Close(); _ = upstream.Close(); _ = application.Close() }()
			gate := &pendingClose{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
			a, b := local, upstream
			if target == "local" {
				a = pendingCloseConn{Conn: local, gate: gate}
			} else {
				b = pendingCloseConn{Conn: upstream, gate: gate}
			}
			done := make(chan error, 1)
			go func() { done <- Relay(ctx, a, b) }()
			cancel()
			assertCloseCompletion(t, "Relay", gate, done)
		})
	}
}

func assertCloseCompletion(t *testing.T, operation string, gate *pendingClose, done <-chan error) {
	t.Helper()
	var err error
	select {
	case <-gate.started:
	case <-time.After(time.Second):
		close(gate.release)
		t.Fatal("cancellation did not close socket")
	}
	returned := false
	select {
	case err = <-done:
		returned = true
		t.Error(operation + " returned before the owned Close completed")
	case <-time.After(100 * time.Millisecond):
	}
	close(gate.release)
	if !returned {
		select {
		case err = <-done:
		case <-time.After(time.Second):
			t.Fatal(operation + " did not return after Close completed")
		}
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation cause lost", err)
	}
	select {
	case <-gate.finished:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish")
	}
}
