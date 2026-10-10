package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestServeWaitsForAcceptedTransportClose(t *testing.T) {
	for _, closingFirst := range []bool{false, true} {
		name := "shutdown-starts-close"
		if closingFirst {
			name = "handler-already-closing"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testServeWaitsForAcceptedClose(t, closingFirst) })
		})
	}
}

func testServeWaitsForAcceptedClose(t *testing.T, closingFirst bool) {
	run := newShutdownServe(t, NewServer(), nil)
	conn, peer := run.pipe()
	gate, release := shutdownGate(t)
	gated := &shutdownGatedConn{shutdownTestConn: conn, entered: make(chan struct{}), gate: gate}
	run.listener.accepted <- gated
	synctest.Wait()
	if conn.readCalls.Load() != 1 {
		t.Fatal("connection did not reach its request read")
	}
	if closingFirst {
		_ = peer.Close()
		synctest.Wait()
		shutdownRequireSignal(t, gated.entered, "handler did not start closing its connection")
	}
	_ = run.listener.Close()
	synctest.Wait()
	shutdownRequireSignal(t, gated.entered, "shutdown did not start closing its connection")
	run.requireRunning(t, "Serve returned before accepted transport Close finished")
	release()
	synctest.Wait()
	run.requireReturned(t, nil)
	shutdownRequireSignal(t, conn.closed, "Serve returned before its transport closed")
}

func TestServeJoinsCancellationWatcherClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := newShutdownTestListener()
		gate := make(chan struct{})
		gated := &shutdownGatedListener{shutdownTestListener: listener, entered: make(chan struct{}), finished: make(chan struct{}), gate: gate}
		run := newShutdownServe(t, NewServer(), gated)
		// Release before the run's cleanup closes this listener again, even on failure.
		release := sync.OnceFunc(func() { close(gate) })
		t.Cleanup(release)
		synctest.Wait()
		run.cancel()
		synctest.Wait()
		shutdownRequireSignal(t, gated.entered, "cancellation watcher did not enter listener.Close")
		run.requireRunning(t, "Serve returned while its cancellation watcher was still in listener.Close")
		release()
		synctest.Wait()
		run.requireReturned(t, context.Canceled)
		shutdownRequireSignal(t, gated.finished, "Serve did not join listener.Close")
	})
}

func TestServeShutdownDoesNotAffectOtherInvocationOrSession(t *testing.T) {
	for _, cancelFirst := range []bool{false, true} {
		name := "listener-close"
		if cancelFirst {
			name = "context-cancel"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testServeOtherInvocation(t, cancelFirst) })
		})
	}
}

func testServeOtherInvocation(t *testing.T, cancelFirst bool) {
	server := NewServer()
	streamContext := make(chan context.Context, 1)
	if err := server.RegisterStream("session", func(context.Context, json.RawMessage) (Stream, error) {
		return func(ctx context.Context, conn net.Conn) error {
			streamContext <- ctx
			_, _ = io.Copy(io.Discard, conn)
			return &SessionExitError{Code: 7}
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	first := newShutdownServe(t, server, nil)
	second := newShutdownServe(t, server, nil)
	firstConn, _ := first.acceptPipe()
	secondConn, peer := second.acceptPipe()
	shutdownWrite(t, peer, `{"version":1,"method":"session","stream":true,"session":true}`+"\n")
	response := shutdownResponse(t, peer)
	if response.SessionID == "" || response.Error != nil {
		t.Fatalf("session handshake = %+v", response)
	}
	synctest.Wait()
	var sessionCtx context.Context
	select {
	case sessionCtx = <-streamContext:
	default:
		t.Fatal("second Serve did not start its session")
	}
	var wantErr error
	if cancelFirst {
		first.cancel()
		wantErr = context.Canceled
	} else {
		_ = first.listener.Close()
	}
	synctest.Wait()
	first.requireReturned(t, wantErr)
	shutdownRequireSignal(t, firstConn.closed, "first Serve did not close its transport")
	second.requireRunning(t, "stopping first Serve stopped second Serve")
	select {
	case <-secondConn.closed:
		t.Fatal("stopping first Serve closed second Serve's transport")
	default:
	}
	if sessionCtx.Err() != nil || second.ctx.Err() != nil {
		t.Fatal("stopping first Serve canceled second Serve's session")
	}
	server.sessionMu.Lock()
	retained := server.sessions[response.SessionID] != nil
	server.sessionMu.Unlock()
	if !retained {
		t.Fatal("stopping first Serve discarded second Serve's session")
	}
	_ = peer.Close()
	synctest.Wait()
	shutdownRequireSignal(t, secondConn.closed, "completed session did not close its transport")
	_ = second.listener.Close()
	synctest.Wait()
	second.requireReturned(t, nil)
	result, err := server.waitSession(context.Background(), response.SessionID)
	if err != nil || result.ExitCode != 7 {
		t.Fatalf("completed session after both Serve calls returned = (%+v, %v), want exit 7", result, err)
	}
}

func TestServeSharesCapacityUntilHandlerActuallyReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := NewServer()
		first := newShutdownServe(t, server, nil)
		second := newShutdownServe(t, server, nil)
		held, _, release := startShutdownBlockedHandler(t, first, false)
		for i := 1; i < maxConcurrentConnections; i++ {
			second.acceptPipe()
		}
		synctest.Wait()
		if got := len(server.connections); got != maxConcurrentConnections {
			t.Fatalf("occupied shared slots = %d, want %d", got, maxConcurrentConnections)
		}
		overflow, _ := second.acceptPipe()
		synctest.Wait()
		shutdownRequireSignal(t, overflow.closed, "other Serve exceeded shared connection capacity")
		if overflow.readCalls.Load() != 0 {
			t.Fatal("capacity-rejected connection was dispatched")
		}
		first.cancel()
		synctest.Wait()
		first.requireReturned(t, context.Canceled)
		shutdownRequireSignal(t, held.closed, "Serve left a noncooperative handler's transport open")
		if got := len(server.connections); got != maxConcurrentConnections {
			t.Fatalf("transport shutdown prematurely released a running handler's slot: %d", got)
		}
		stillOverflow, _ := second.acceptPipe()
		synctest.Wait()
		shutdownRequireSignal(t, stillOverflow.closed, "running handler no longer counted against capacity")
		release()
		synctest.Wait()
		if got := len(server.connections); got != maxConcurrentConnections-1 {
			t.Fatalf("returned handler did not release exactly its slot: %d", got)
		}
		replacement, _ := second.acceptPipe()
		synctest.Wait()
		if replacement.readCalls.Load() != 1 || len(server.connections) != maxConcurrentConnections {
			t.Fatal("released capacity could not admit a new connection")
		}
	})
}

func TestServeOwnsSuccessfulAcceptRacingShutdown(t *testing.T) {
	for _, cancelFirst := range []bool{false, true} {
		name := "listener-close"
		if cancelFirst {
			name = "context-cancel"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testServeLateAccept(t, cancelFirst) })
		})
	}
}

func testServeLateAccept(t *testing.T, cancelFirst bool) {
	listener := newShutdownTestListener()
	gate := make(chan struct{})
	late := &shutdownLateAcceptListener{shutdownTestListener: listener, captured: make(chan struct{}), gate: gate}
	run := newShutdownServe(t, NewServer(), late)
	release := sync.OnceFunc(func() { close(gate) })
	t.Cleanup(release)
	conn, peer := run.acceptPipe()
	synctest.Wait()
	shutdownRequireSignal(t, late.captured, "listener did not capture successful accept")
	var wantErr error
	if cancelFirst {
		run.cancel()
		wantErr = context.Canceled
	} else {
		_ = listener.Close()
	}
	synctest.Wait()
	shutdownRequireSignal(t, listener.closed, "listener shutdown did not precede successful Accept return")
	release()
	synctest.Wait()
	run.requireReturned(t, wantErr)
	shutdownRequireSignal(t, conn.closed, "successful Accept racing shutdown escaped transport ownership")
	shutdownEOF(t, peer)
}

func TestServeSupportsUncomparableConnectionValues(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newShutdownServe(t, NewServer(), nil)
		conn, peer := run.pipe()
		run.listener.accepted <- shutdownUncomparableConn{Conn: conn, marker: []byte{1}}
		synctest.Wait()
		if conn.readCalls.Load() != 1 {
			t.Fatal("uncomparable connection was not dispatched")
		}
		run.cancel()
		synctest.Wait()
		run.requireReturned(t, context.Canceled)
		shutdownRequireSignal(t, conn.closed, "uncomparable connection was not closed")
		shutdownEOF(t, peer)
	})
}

func TestServePreservesHandlerContextAfterOrdinaryDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rawStream   bool
		acceptError bool
	}{
		{name: "call/listener-close"},
		{name: "call/accept-error", acceptError: true},
		{name: "raw-stream/listener-close", rawStream: true},
		{name: "raw-stream/accept-error", rawStream: true, acceptError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testServeHandlerContext(t, tc.rawStream, tc.acceptError) })
		})
	}
}

func testServeHandlerContext(t *testing.T, rawStream, acceptError bool) {
	server := NewServer()
	run := newShutdownServe(t, server, nil)
	conn, peer, release := startShutdownBlockedHandler(t, run, rawStream)
	_ = peer.Close()
	synctest.Wait()
	if run.ctx.Err() != nil || len(server.connections) != 1 {
		t.Fatal("ordinary client disconnect canceled or released a still-running handler")
	}
	run.requireRunning(t, "ordinary client disconnect stopped Serve")
	var wantErr error
	if acceptError {
		wantErr = errors.New("scripted accept failure")
		run.listener.failed <- wantErr
	} else {
		_ = run.listener.Close()
	}
	synctest.Wait()
	run.requireReturned(t, wantErr)
	if run.ctx.Err() != nil {
		t.Fatal("listener shutdown canceled the caller's handler context")
	}
	shutdownRequireSignal(t, conn.closed, "listener shutdown did not close the handler transport")
	release()
	synctest.Wait()
	if len(server.connections) != 0 {
		t.Fatal("completed handler did not release its slot")
	}
}

// Start a handler that retains its connection slot until explicitly released,
// even after cancellation or disconnect. Verify the original context at both
// stream preparation and execution boundaries, or at the unary call boundary.
func startShutdownBlockedHandler(t *testing.T, run *shutdownServe, rawStream bool) (*shutdownTestConn, net.Conn, func()) {
	t.Helper()
	gate, release := shutdownGate(t)
	received := make(chan context.Context, 2)
	var err error
	request := `{"version":1,"method":"blocked"}`
	if rawStream {
		request = `{"version":1,"method":"blocked","stream":true}`
		err = run.server.RegisterStream("blocked", func(ctx context.Context, _ json.RawMessage) (Stream, error) {
			received <- ctx
			return func(ctx context.Context, _ net.Conn) error {
				received <- ctx
				<-gate
				return nil
			}, nil
		})
	} else {
		err = run.server.Register("blocked", func(ctx context.Context, _ json.RawMessage) (any, error) {
			received <- ctx
			<-gate
			return nil, nil
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	conn, peer := run.acceptPipe()
	shutdownWrite(t, peer, request+"\n")
	if rawStream {
		if response := shutdownResponse(t, peer); response.Error != nil {
			t.Fatalf("stream handshake = %+v", response)
		}
	}
	synctest.Wait()
	wantContexts := 1
	if rawStream {
		wantContexts = 2
	}
	for i := 0; i < wantContexts; i++ {
		select {
		case ctx := <-received:
			if ctx != run.ctx {
				t.Fatal("handler or raw stream received a replacement context")
			}
		default:
			t.Fatal("handler or raw stream did not start")
		}
	}
	return conn, peer, release
}

func TestServeShutdownDiscardsBlockedSessionHandshake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := NewServer()
		var started atomic.Bool
		if err := server.RegisterStream("session", func(context.Context, json.RawMessage) (Stream, error) {
			return func(context.Context, net.Conn) error { started.Store(true); return nil }, nil
		}); err != nil {
			t.Fatal(err)
		}
		run := newShutdownServe(t, server, nil)
		conn, peer := run.acceptPipe()
		shutdownWrite(t, peer, `{"version":1,"method":"session","stream":true,"session":true}`+"\n")
		server.sessionMu.Lock()
		sessions := len(server.sessions)
		server.sessionMu.Unlock()
		if conn.writeCalls.Load() != 1 || sessions != 1 || started.Load() {
			t.Fatal("fixture did not establish a blocked session handshake")
		}
		_ = run.listener.Close()
		synctest.Wait()
		run.requireReturned(t, nil)
		shutdownRequireSignal(t, conn.closed, "blocked handshake transport stayed open")
		server.sessionMu.Lock()
		sessions = len(server.sessions)
		server.sessionMu.Unlock()
		if sessions != 0 || started.Load() {
			t.Fatal("shutdown retained or started an unacknowledged session")
		}
	})
}

func TestServeDoesNotCloseCompletedConnectionsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newShutdownServe(t, NewServer(), nil)
		conn, peer := run.acceptPipe()
		synctest.Wait()
		_ = peer.Close()
		synctest.Wait()
		shutdownRequireSignal(t, conn.closed, "ordinary disconnect did not complete connection cleanup")
		if conn.closeCalls.Load() != 1 || len(run.server.connections) != 0 {
			t.Fatal("completed connection did not finish its normal cleanup")
		}
		_ = run.listener.Close()
		synctest.Wait()
		run.requireReturned(t, nil)
		if got := conn.closeCalls.Load(); got != 1 {
			t.Fatalf("shutdown retained an already-completed connection: Close called %d times", got)
		}
	})
}

func TestServeConnectionCompletionProgressesDuringShutdownClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newShutdownServe(t, NewServer(), nil)
		conn, _ := run.pipe()
		gate, release := shutdownGate(t)
		gated := &shutdownGatedConn{shutdownTestConn: conn, entered: make(chan struct{}), gate: gate}
		run.listener.accepted <- gated
		other, peer := run.acceptPipe()
		synctest.Wait()
		if conn.readCalls.Load() != 1 || other.readCalls.Load() != 1 {
			t.Fatal("connections did not reach their request reads")
		}
		_ = run.listener.Close()
		synctest.Wait()
		shutdownRequireSignal(t, gated.entered, "shutdown did not reach gated Close")
		// Whether shutdown closed this transport before or after reaching the
		// gate, ordinary completion must be able to unregister and release it.
		_ = peer.Close()
		synctest.Wait()
		shutdownRequireSignal(t, other.closed, "other connection did not finish closing")
		if got := len(run.server.connections); got != 1 {
			t.Fatalf("connection completion blocked behind another transport's Close: %d occupied slots", got)
		}
		run.requireRunning(t, "Serve returned before gated Close finished")
		release()
		synctest.Wait()
		run.requireReturned(t, nil)
		if len(run.server.connections) != 0 {
			t.Fatal("shutdown did not release finished connections")
		}
	})
}

// All cleanup is registered inside the synctest bubble. Raw pipe ends are closed
// even when assertions fail; gate releases are registered after this cleanup so
// deliberately blocked handlers and Close calls are released first.
type shutdownServe struct {
	server   *Server
	ctx      context.Context
	cancel   context.CancelFunc
	listener *shutdownTestListener
	done     chan error
	pipes    []net.Conn
}

func newShutdownServe(t *testing.T, server *Server, listener net.Listener) *shutdownServe {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &shutdownServe{server: server, ctx: ctx, cancel: cancel, done: make(chan error, 1)}
	switch l := listener.(type) {
	case nil:
		run.listener = newShutdownTestListener()
		listener = run.listener
	case *shutdownGatedListener:
		run.listener = l.shutdownTestListener
	case *shutdownLateAcceptListener:
		run.listener = l.shutdownTestListener
	default:
		t.Fatalf("unsupported test listener %T", listener)
	}
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		for _, conn := range run.pipes {
			_ = conn.Close()
		}
		synctest.Wait()
	})
	go func() { run.done <- server.Serve(ctx, listener) }()
	return run
}

func (r *shutdownServe) pipe() (*shutdownTestConn, net.Conn) {
	wire, peer := net.Pipe()
	r.pipes = append(r.pipes, wire, peer)
	return &shutdownTestConn{Conn: wire, closed: make(chan struct{})}, peer
}

func (r *shutdownServe) acceptPipe() (*shutdownTestConn, net.Conn) {
	conn, peer := r.pipe()
	r.listener.accepted <- conn
	return conn, peer
}

func (r *shutdownServe) stop(reason string) error {
	switch reason {
	case "context-cancel":
		r.cancel()
		return context.Canceled
	case "accept-error":
		err := errors.New("scripted accept failure")
		r.listener.failed <- err
		return err
	default:
		_ = r.listener.Close()
		return nil
	}
}

func (r *shutdownServe) requireReturned(t *testing.T, want error) {
	t.Helper()
	select {
	case err := <-r.done:
		if !errors.Is(err, want) {
			t.Fatalf("Serve returned %v, want %v", err, want)
		}
	default:
		t.Fatal("Serve has not returned")
	}
}

func (r *shutdownServe) requireRunning(t *testing.T, message string) {
	t.Helper()
	select {
	case err := <-r.done:
		t.Fatalf("%s (returned %v)", message, err)
	default:
	}
}

func shutdownGate(t *testing.T) (<-chan struct{}, func()) {
	t.Helper()
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	t.Cleanup(release)
	return gate, release
}

func shutdownRequireSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	default:
		t.Fatal(message)
	}
}

func shutdownWrite(t *testing.T, peer net.Conn, request string) {
	t.Helper()
	done := make(chan error, 1)
	go func() { _, err := io.WriteString(peer, request); done <- err }()
	synctest.Wait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("request write: %v", err)
		}
	default:
		t.Fatal("request write did not finish")
	}
}

func shutdownResponse(t *testing.T, peer net.Conn) responseEnvelope {
	t.Helper()
	var response responseEnvelope
	done := make(chan error, 1)
	go func() { done <- json.NewDecoder(peer).Decode(&response) }()
	synctest.Wait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("response read: %v", err)
		}
	default:
		t.Fatal("response read did not finish")
	}
	return response
}

func shutdownEOF(t *testing.T, peer net.Conn) {
	t.Helper()
	done := make(chan shutdownTestRead, 1)
	go func() {
		var b [1]byte
		n, err := peer.Read(b[:])
		done <- shutdownTestRead{n: n, err: err}
	}()
	synctest.Wait()
	select {
	case result := <-done:
		if result.n != 0 || !errors.Is(result.err, io.EOF) {
			t.Fatalf("peer read = (%d, %v), want (0, EOF)", result.n, result.err)
		}
	default:
		t.Fatal("peer did not observe EOF")
	}
}

type shutdownGatedConn struct {
	*shutdownTestConn
	entered chan struct{}
	gate    <-chan struct{}
	once    sync.Once
}

// The handler's deferred Close and shutdown Close may overlap, as allowed by
// net.Conn's concurrent-call contract. Both must finish before their callers do.
func (c *shutdownGatedConn) Close() error {
	c.once.Do(func() { close(c.entered) })
	<-c.gate
	return c.shutdownTestConn.Close()
}

type shutdownGatedListener struct {
	*shutdownTestListener
	entered  chan struct{}
	finished chan struct{}
	gate     <-chan struct{}
	enter    sync.Once
	finish   sync.Once
}

func (l *shutdownGatedListener) Close() error {
	err := l.shutdownTestListener.Close() // Unblock Accept before Close returns.
	l.enter.Do(func() { close(l.entered) })
	<-l.gate
	l.finish.Do(func() { close(l.finished) })
	return err
}

type shutdownLateAcceptListener struct {
	*shutdownTestListener
	captured chan struct{}
	gate     <-chan struct{}
	once     sync.Once
}

func (l *shutdownLateAcceptListener) Accept() (net.Conn, error) {
	conn, err := l.shutdownTestListener.Accept()
	if err == nil {
		l.once.Do(func() {
			close(l.captured)
			<-l.gate
		})
	}
	return conn, err
}

type shutdownUncomparableConn struct {
	net.Conn
	marker []byte
}
