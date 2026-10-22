package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

// This characterizes existing ordering, not a WSL crash: application EOF can
// precede a separate completion call, which still belongs to the tunnel owner.
func TestForwardCompletionCancellationAfterApplicationEOF(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	f := newForwardCompletionFixture(t, ctx)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	user, served, observed := f.serve(t, runCtx)
	forwardCompletionExchange(t, user)
	awaitForwardCompletion(t, ctx, f.applicationDone, "application completion")
	if !receiveForwardCompletion(t, ctx, f.ordered, "completion dial") {
		t.Fatal("completion dial started before data transport Close finished")
	}
	if method := receiveForwardCompletion(t, ctx, f.method, "completion request"); method != "_control.session.wait" {
		t.Fatalf("unexpected second operation: %q", method)
	}
	select {
	case err := <-observed:
		t.Fatalf("completion returned before its peer answered or cancellation: %v", err)
	default:
	}
	cancel()
	if err := receiveForwardCompletion(t, ctx, served, "Serve cancellation"); !errors.Is(err, context.Canceled) {
		t.Fatal("Serve lost owner cancellation", err)
	}
	awaitForwardCompletion(t, ctx, f.completionClosed, "completion transport closure")
	select {
	case err := <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("completion call lost owner cancellation", err)
		}
	default:
		t.Fatal("Serve returned before the completion worker reported its result")
	}
	if err := receiveForwardCompletion(t, ctx, f.peerDone, "completion peer exit"); err != nil {
		t.Fatal("completion transport retained its peer", err)
	}
	if got := f.dials.Load(); got != 2 {
		t.Fatalf("completion cancellation retried or opened another operation: %d dials", got)
	}
}

func TestForwardCompletionFixtureCleanupWithoutOwnerCancellation(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	f := newForwardCompletionFixture(t, ctx)
	user, served, _ := f.serve(t, ctx)
	forwardCompletionExchange(t, user)
	receiveForwardCompletion(t, ctx, f.method, "held completion request")
	f.cleanup(t)
	if err := ctx.Err(); err != nil {
		t.Fatal("fixture cleanup depended on owner cancellation or deadline", err)
	}
	select {
	case <-served:
	default:
		t.Fatal("independent fixture cleanup did not join Serve")
	}
	select {
	case <-f.peerDone:
	default:
		t.Fatal("independent fixture cleanup did not join the completion peer")
	}
}

type forwardCompletionFixture struct {
	client           *Client
	controller       *terminalTestListener
	stop             context.CancelFunc
	cleanupMu        sync.Mutex
	cleanupStarted   chan struct{}
	closing          bool
	transports       []io.Closer
	workers          []<-chan struct{}
	dials            atomic.Int32
	applicationDone  chan struct{}
	dataClosed       chan struct{}
	completionClosed chan struct{}
	ordered          chan bool
	method           chan string
	peerDone         chan error
}

func newForwardCompletionFixture(t *testing.T, ctx context.Context) *forwardCompletionFixture {
	t.Helper()
	ctx, stop := context.WithCancel(ctx)
	f := &forwardCompletionFixture{
		controller:       &terminalTestListener{connections: make(chan net.Conn), done: make(chan struct{})},
		stop:             stop,
		cleanupStarted:   make(chan struct{}),
		applicationDone:  make(chan struct{}),
		dataClosed:       make(chan struct{}),
		completionClosed: make(chan struct{}),
		ordered:          make(chan bool, 1),
		method:           make(chan string, 1),
		peerDone:         make(chan error, 1),
	}
	t.Cleanup(func() { f.cleanup(t) })
	f.ownTransports(f.controller)
	server := control.NewServer()
	if err := server.RegisterStream(MethodForwardStream, f.application); err != nil {
		t.Fatal(err)
	}
	f.startWorker(func() { _ = server.Serve(ctx, f.controller) })
	var err error
	f.client, err = NewClientWithDialer(f.dial)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *forwardCompletionFixture) application(context.Context, json.RawMessage) (control.Stream, error) {
	return func(_ context.Context, conn net.Conn) error {
		defer close(f.applicationDone)
		if err := json.NewEncoder(conn).Encode(forwardReady{Ready: true}); err != nil {
			return err
		}
		data, err := io.ReadAll(conn)
		if err != nil {
			return err
		}
		if _, err = conn.Write(data); err != nil {
			return err
		}
		return conn.(interface{ CloseWrite() error }).CloseWrite()
	}, nil
}

func (f *forwardCompletionFixture) dial(ctx context.Context) (net.Conn, error) {
	switch f.dials.Add(1) {
	case 1:
		local, remote := f.pipe(true)
		select {
		case f.controller.connections <- remote:
			return &forwardCompletionConn{Conn: local, closed: f.dataClosed}, nil
		case <-ctx.Done():
			_ = local.Close()
			_ = remote.Close()
			return nil, ctx.Err()
		case <-f.cleanupStarted:
			return nil, net.ErrClosed
		}
	case 2:
		select {
		case <-f.dataClosed:
			f.ordered <- true
		default:
			f.ordered <- false
		}
		local, remote := f.pipe(false)
		if !f.startWorker(func() { f.holdCompletion(remote) }) {
			return nil, net.ErrClosed
		}
		return &forwardCompletionConn{Conn: local, closed: f.completionClosed}, nil
	default:
		return nil, errors.New("unexpected extra completion-fixture dial")
	}
}

func (f *forwardCompletionFixture) holdCompletion(peer net.Conn) {
	defer func() { _ = peer.Close() }()
	var request struct{ Method string }
	if err := json.NewDecoder(peer).Decode(&request); err != nil {
		f.peerDone <- err
		return
	}
	f.method <- request.Method
	// Never answer the completion request. Only owner cancellation may release it.
	_, err := io.Copy(io.Discard, peer)
	f.peerDone <- err
}

func (f *forwardCompletionFixture) serve(t *testing.T, ctx context.Context) (net.Conn, <-chan error, <-chan error) {
	t.Helper()
	user, local := f.pipe(true)
	listener := forwardCompletionListener{&terminalTestListener{connections: make(chan net.Conn, 1), done: make(chan struct{})}}
	listener.connections <- local
	f.ownTransports(listener)
	served, observed := make(chan error, 1), make(chan error, 1)
	selected := core.EnvironmentTCPForward{Environment: "demo", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: "127.0.0.1", Port: 80}
	f.startWorker(func() {
		served <- streamio.Serve(ctx, listener, func(ctx context.Context) (net.Conn, error) {
			return f.client.OpenEnvironmentForward(ctx, selected)
		}, func(err error) { observed <- err })
	})
	return user, served, observed
}

func forwardCompletionExchange(t *testing.T, user net.Conn) {
	t.Helper()
	if err := user.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(user, "completed application bytes"); err != nil {
		t.Fatal(err)
	}
	if err := user.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(user)
	if err != nil || string(data) != "completed application bytes" {
		t.Fatalf("application exchange: %q, %v", data, err)
	}
}

func (f *forwardCompletionFixture) pipe(framed bool) (net.Conn, net.Conn) {
	a, b := net.Pipe()
	// Keep raw endpoints so fatal-path cleanup bypasses the product Close and
	// cancellation behavior under test, including a held completion response.
	f.ownTransports(a, b)
	if !framed {
		return a, b
	}
	return streamio.NewFramedConn(a, a), streamio.NewFramedConn(b, b)
}

func (f *forwardCompletionFixture) ownTransports(transports ...io.Closer) {
	f.cleanupMu.Lock()
	defer f.cleanupMu.Unlock()
	if f.closing {
		for _, transport := range transports {
			_ = transport.Close()
		}
		return
	}
	f.transports = append(f.transports, transports...)
}

func (f *forwardCompletionFixture) startWorker(run func()) bool {
	f.cleanupMu.Lock()
	defer f.cleanupMu.Unlock()
	if f.closing {
		return false
	}
	done := make(chan struct{})
	f.workers = append(f.workers, done)
	go func() { defer close(done); run() }()
	return true
}

func (f *forwardCompletionFixture) cleanup(t *testing.T) {
	t.Helper()
	f.cleanupMu.Lock()
	if !f.closing {
		f.closing = true
		close(f.cleanupStarted)
	}
	transports, workers := f.transports, f.workers
	f.cleanupMu.Unlock()
	f.stop()
	for _, transport := range transports {
		_ = transport.Close()
	}
	// Completion channels are separate from test result channels: cleanup joins
	// every started fixture worker even after the happy path consumed its result.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for _, done := range workers {
		select {
		case <-done:
		case <-deadline.C:
			t.Error("fixture worker survived independent transport cleanup")
			return
		}
	}
}

type forwardCompletionListener struct{ *terminalTestListener }

func (forwardCompletionListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
}

type forwardCompletionConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
	err    error
}

func (c *forwardCompletionConn) Close() error {
	c.once.Do(func() { c.err = c.Conn.Close(); close(c.closed) })
	return c.err
}

func (c *forwardCompletionConn) CloseWrite() error {
	return c.Conn.(interface{ CloseWrite() error }).CloseWrite()
}

func awaitForwardCompletion(t *testing.T, ctx context.Context, ready <-chan struct{}, description string) {
	t.Helper()
	receiveForwardCompletion(t, ctx, ready, description)
}

func receiveForwardCompletion[T any](t *testing.T, ctx context.Context, ready <-chan T, description string) T {
	t.Helper()
	select {
	case value := <-ready:
		return value
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", description, ctx.Err())
		var zero T
		return zero
	}
}
