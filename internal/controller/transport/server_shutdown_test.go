package control

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// These tests use only in-memory transports. synctest establishes blocked
// protocol states without sleeps and checks that every owned goroutine exits.
func TestServeClosesAcceptedTransportsBeforeReturning(t *testing.T) {
	for _, stop := range []string{"context-cancel", "listener-close", "accept-error"} {
		for _, state := range []string{"idle-envelope", "partial-envelope", "blocked-response"} {
			t.Run(stop+"/"+state, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) { testServeClosesTransport(t, stop, state) })
			})
		}
	}
}

func testServeClosesTransport(t *testing.T, stop, state string) {
	server := NewServer()
	// Registered before run cleanup, so this assertion observes its final state.
	t.Cleanup(func() {
		if len(server.connections) != 0 {
			t.Error("test cleanup did not release its accepted connection slot")
		}
	})
	run := newShutdownServe(t, server, nil)
	conn, peer := run.acceptPipe()
	synctest.Wait()
	if conn.readCalls.Load() != 1 || run.listener.acceptCalls.Load() != 2 {
		t.Fatal("fixture did not establish accepted connection blocked in request read")
	}
	if state != "idle-envelope" {
		request := `{"version":1,"method":"absent"}`
		if state == "blocked-response" {
			request += "\n"
		}
		shutdownWrite(t, peer, request)
		if state == "partial-envelope" && conn.readCalls.Load() != 2 {
			t.Fatal("fixture did not establish incomplete envelope read")
		}
		if state == "blocked-response" && conn.writeCalls.Load() != 1 {
			t.Fatal("fixture did not establish blocked response write")
		}
	}
	wantErr := run.stop(stop)
	synctest.Wait()
	run.requireReturned(t, wantErr)
	shutdownRequireSignal(t, conn.closed, "Serve returned without closing its accepted transport")
	shutdownEOF(t, peer)
}

func TestServeCancellationWatcherEndsWithServe(t *testing.T) {
	for _, stop := range []string{"listener-close", "accept-error"} {
		t.Run(stop, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testServeWatcherStops(t, stop) })
		})
	}
}

func testServeWatcherStops(t *testing.T, stop string) {
	run := newShutdownServe(t, NewServer(), nil)
	synctest.Wait()
	wantErr := run.stop(stop)
	synctest.Wait()
	run.requireReturned(t, wantErr)
	if run.ctx.Err() != nil {
		t.Fatal("fixture canceled the parent before observing Serve return")
	}
	closedAtReturn := run.listener.closeCalls.Load()
	run.cancel()
	synctest.Wait()
	if got := run.listener.closeCalls.Load(); got != closedAtReturn {
		t.Errorf("watcher outlived Serve: parent cancellation after return called listener.Close (%d -> %d)", closedAtReturn, got)
	}
}

type shutdownTestRead struct {
	n   int
	err error
}

type shutdownTestConn struct {
	net.Conn
	closed     chan struct{}
	closeOnce  sync.Once
	readCalls  atomic.Int32
	writeCalls atomic.Int32
	closeCalls atomic.Int32
}

func (c *shutdownTestConn) Read(p []byte) (int, error) {
	c.readCalls.Add(1)
	return c.Conn.Read(p)
}

func (c *shutdownTestConn) Write(p []byte) (int, error) {
	c.writeCalls.Add(1)
	return c.Conn.Write(p)
}

func (c *shutdownTestConn) Close() error {
	c.closeCalls.Add(1)
	err := c.Conn.Close()
	c.closeOnce.Do(func() { close(c.closed) })
	return err
}

type shutdownTestListener struct {
	accepted    chan net.Conn
	failed      chan error
	closed      chan struct{}
	closeOnce   sync.Once
	acceptCalls atomic.Int32
	closeCalls  atomic.Int32
}

func newShutdownTestListener() *shutdownTestListener {
	return &shutdownTestListener{
		accepted: make(chan net.Conn, 1),
		failed:   make(chan error, 1),
		closed:   make(chan struct{}),
	}
}

func (l *shutdownTestListener) Accept() (net.Conn, error) {
	l.acceptCalls.Add(1)
	select {
	case conn := <-l.accepted:
		return conn, nil
	case err := <-l.failed:
		return nil, err
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *shutdownTestListener) Close() error {
	l.closeCalls.Add(1)
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *shutdownTestListener) Addr() net.Addr { return shutdownTestAddr{} }

type shutdownTestAddr struct{}

func (shutdownTestAddr) Network() string { return "pipe" }
func (shutdownTestAddr) String() string  { return "shutdown-test" }
