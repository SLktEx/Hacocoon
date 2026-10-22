package controlapi

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
)

func TestSetupTransportLifetimeKeepsExclusionUntilServiceReturns(t *testing.T) {
	for _, progress := range []bool{false, true} {
		mode := "rpc"
		if progress {
			mode = "progress"
		}
		for _, stop := range []string{"client-disconnect", "listener-close", "parent-cancel"} {
			t.Run(mode+"/"+stop, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					testSetupTransportLifetime(t, progress, stop)
				})
			})
		}
	}
}

func testSetupTransportLifetime(t *testing.T, progress bool, stop string) {
	t.Helper()
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	defer finish()
	var calls atomic.Int32
	server := control.NewServer()
	if err := RegisterSetup(server, setupServiceFunc(func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			entered <- ctx
			// Cancellation is a signal, not proof that setup has finished.
			<-release
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	client, listener, serveDone := setupTransportClient(t, server, parent)
	observer, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	call := func(c *Client, ctx context.Context) error {
		if progress {
			return c.SetupHostProgress(ctx, recipes.Update{}, nil)
		}
		return c.SetupHost(ctx, recipes.Update{})
	}
	observed := make(chan error, 1)
	go func() { observed <- call(client, observer) }()
	synctest.Wait()
	var setupCtx context.Context
	select {
	case setupCtx = <-entered:
	default:
		t.Fatal("setup service did not start")
	}
	if deadline, ok := setupCtx.Deadline(); !ok || time.Until(deadline) != setupTimeout {
		t.Fatal("setup did not retain its server-owned timeout")
	}

	switch stop {
	case "client-disconnect":
		disconnect()
	case "listener-close":
		_ = listener.Close()
	case "parent-cancel":
		cancelParent()
	}
	synctest.Wait()
	checkSetupTransportStopped(t, stop, serveDone, observed, setupCtx)
	if stop != "client-disconnect" {
		// A second listener shares setup exclusion, not the old Serve lifetime.
		client, _, _ = setupTransportClient(t, server, context.Background())
	}
	checkSetupExclusion(t, client, &calls)
	if stop == "listener-close" {
		// Serve returning must not detach setup from its original parent.
		cancelParent()
		synctest.Wait()
		if !errors.Is(setupCtx.Err(), context.Canceled) {
			t.Fatalf("parent cancellation after Serve returned did not reach setup: %v", setupCtx.Err())
		}
	}
	finish()
	synctest.Wait()
	if err := call(client, context.Background()); err != nil {
		t.Fatalf("explicit retry after service completion: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("setup service calls after retry = %d, want 2", calls.Load())
	}
}

func checkSetupExclusion(t *testing.T, client *Client, calls *atomic.Int32) {
	t.Helper()
	// Both entry points must reject another mutation until the service returns.
	for _, err := range []error{
		client.SetupHost(context.Background(), recipes.Update{}),
		client.SetupHostProgress(context.Background(), recipes.Update{}, nil),
	} {
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "busy" {
			t.Fatalf("unfinished setup lost exclusion: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("setup service calls before release = %d, want 1", calls.Load())
	}
}

func checkSetupTransportStopped(t *testing.T, stop string, serveDone, observed <-chan error, setupCtx context.Context) {
	t.Helper()
	var wantContextErr error
	if stop == "parent-cancel" {
		wantContextErr = context.Canceled
	}
	if stop != "client-disconnect" {
		checkSetupServeReturned(t, serveDone, wantContextErr)
	}
	select {
	case err := <-observed:
		if err == nil {
			t.Fatal("lost observation was reported as setup success")
		}
		if stop == "client-disconnect" && !errors.Is(err, context.Canceled) {
			t.Fatalf("client cancellation returned %v", err)
		}
	default:
		t.Fatal("setup observation remained blocked after transport shutdown")
	}
	if !errors.Is(setupCtx.Err(), wantContextErr) {
		t.Fatalf("setup context = %v, want %v", setupCtx.Err(), wantContextErr)
	}
}

func checkSetupServeReturned(t *testing.T, serveDone <-chan error, want error) {
	t.Helper()
	select {
	case err := <-serveDone:
		if !errors.Is(err, want) {
			t.Fatalf("Serve returned %v, want %v", err, want)
		}
	default:
		t.Fatal("Serve waited for blocked setup instead of closing transport")
	}
}

// Every dial uses the product client and Server.Serve with a fresh in-memory
// transport. Cleanup closes both endpoints even if an assertion fails early.
func setupTransportClient(t *testing.T, server *control.Server, parent context.Context) (*Client, *setupPipeListener, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(parent)
	listener := &setupPipeListener{accepted: make(chan net.Conn), closed: make(chan struct{})}
	done := make(chan error, 1)
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		listener.mu.Lock()
		for _, conn := range listener.pipes {
			_ = conn.Close()
		}
		listener.mu.Unlock()
		synctest.Wait()
	})
	client, err := NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
		wire, peer := net.Pipe()
		listener.mu.Lock()
		listener.pipes = append(listener.pipes, wire, peer)
		listener.mu.Unlock()
		select {
		case listener.accepted <- wire:
			return peer, nil
		case <-ctx.Done():
			_ = wire.Close()
			_ = peer.Close()
			return nil, ctx.Err()
		case <-listener.closed:
			_ = wire.Close()
			_ = peer.Close()
			return nil, net.ErrClosed
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { done <- server.Serve(ctx, listener) }()
	return client, listener, done
}

type setupPipeListener struct {
	accepted chan net.Conn
	closed   chan struct{}
	once     sync.Once
	mu       sync.Mutex
	pipes    []net.Conn
}

func (l *setupPipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.accepted:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *setupPipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *setupPipeListener) Addr() net.Addr { return setupPipeAddr{} }

type setupPipeAddr struct{}

func (setupPipeAddr) Network() string { return "pipe" }
func (setupPipeAddr) String() string  { return "setup-transport-test" }
