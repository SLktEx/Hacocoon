package networkrelay

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// A network Close can unblock Accept before releasing the underlying socket.
// As with Go's poll.FD, a duplicate Close need not wait for the first one.
type delayedCloseListener struct {
	accepting   chan struct{}
	closing     chan struct{}
	release     chan struct{}
	closed      chan struct{}
	closeCalls  atomic.Int32
	acceptError error
	beforeError func()
	address     net.Addr
}

func newDelayedCloseListener() *delayedCloseListener {
	return &delayedCloseListener{accepting: make(chan struct{}), closing: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
}

func (l *delayedCloseListener) Addr() net.Addr {
	if l.address != nil {
		return l.address
	}
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}

func (l *delayedCloseListener) Accept() (net.Conn, error) {
	close(l.accepting)
	if l.acceptError != nil {
		if l.beforeError != nil {
			l.beforeError()
		}
		return nil, l.acceptError
	}
	<-l.closing
	return nil, net.ErrClosed
}

// Hold the context's AfterFunc propagation so cancellation can be observed by
// Accept before the registered callback starts. This uses Context's documented
// AfterFunc scheduling hook; it does not alter the production listener path.
type heldAfterFuncContext struct {
	context.Context
	done                chan struct{}
	registered, stopped bool
}

func (c *heldAfterFuncContext) Done() <-chan struct{} { return c.done }
func (c *heldAfterFuncContext) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}
func (c *heldAfterFuncContext) AfterFunc(func()) func() bool {
	c.registered = true
	return func() bool { c.stopped = true; return true }
}

func TestServeTCPCancellationBeforeCallbackStillClosesSynchronously(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := &heldAfterFuncContext{Context: context.Background(), done: make(chan struct{})}
		listener := newDelayedCloseListener()
		listener.acceptError = net.ErrClosed
		listener.beforeError = func() { close(ctx.done) }
		done := make(chan error, 1)
		go func() { done <- ServeTCP(ctx, listener, Spec{}, nil, nil) }()
		synctest.Wait()
		select {
		case <-listener.closing:
		default:
			t.Error("cancellation did not begin owned Close")
		}
		var err error
		returned := false
		select {
		case err = <-done:
			returned = true
			t.Error("cancellation returned before synchronous close completed", err)
		default:
		}
		close(listener.release)
		if !returned {
			err = <-done
		}
		if !errors.Is(err, context.Canceled) || !ctx.registered || !ctx.stopped || listener.closeCalls.Load() != 1 {
			t.Fatalf("cancellation result = %v, registered = %v, stopped = %v, close calls = %d", err, ctx.registered, ctx.stopped, listener.closeCalls.Load())
		}
	})
}

func (l *delayedCloseListener) Close() error {
	if l.closeCalls.Add(1) != 1 {
		return net.ErrClosed
	}
	close(l.closing)
	<-l.release
	close(l.closed)
	return nil
}

func TestServeTCPWaitsForCancellationCloseCompletion(t *testing.T) {
	for _, alreadyCanceled := range []bool{false, true} {
		name := "during-accept"
		if alreadyCanceled {
			name = "before-serve"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				listener := newDelayedCloseListener()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if alreadyCanceled {
					cancel()
				}
				done := make(chan error, 1)
				go func() { done <- ServeTCP(ctx, listener, Spec{}, nil, nil) }()
				<-listener.accepting
				cancel()
				<-listener.closing
				synctest.Wait()
				var err error
				returned := false
				select {
				case err = <-done:
					returned = true
					t.Error("ServeTCP returned before cancellation finished closing its listener")
				default:
				}
				close(listener.release)
				if !returned {
					err = <-done
				}
				if !errors.Is(err, context.Canceled) || listener.closeCalls.Load() != 1 {
					t.Fatalf("cancellation result = %v, close calls = %d", err, listener.closeCalls.Load())
				}
				select {
				case <-listener.closed:
				default:
					t.Error("listener Close has not completed")
				}
			})
		})
	}
}

func TestServeTCPAcceptFailureStopsCancellationCallbackWithoutClosingListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := newDelayedCloseListener()
		close(listener.release)
		listener.acceptError = errors.New("independent accept failure")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := ServeTCP(ctx, listener, Spec{}, nil, nil); !errors.Is(err, listener.acceptError) {
			t.Fatal("accept failure changed", err)
		}
		cancel()
		synctest.Wait()
		if listener.closeCalls.Load() != 0 {
			t.Fatal("stopped cancellation callback closed caller-owned listener")
		}
	})
}

func TestServeTCPInvalidListenerRemainsCallerOwned(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := newDelayedCloseListener()
		close(listener.release)
		listener.address = &net.TCPAddr{IP: net.IPv4zero, Port: 1234}
		listener.acceptError = errors.New("unexpected Accept")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := ServeTCP(ctx, listener, Spec{}, nil, nil); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal("invalid listener accepted", err)
		}
		cancel()
		synctest.Wait()
		if listener.closeCalls.Load() != 0 {
			t.Fatal("refusal closed caller-owned listener")
		}
		select {
		case <-listener.accepting:
			t.Fatal("invalid listener reached Accept")
		default:
		}
	})
}
