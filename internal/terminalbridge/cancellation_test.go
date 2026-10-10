package terminalbridge

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

func TestBridgePreservesCallerCancellationBeforeSignalContextPropagation(t *testing.T) {
	for _, outputErr := range []error{
		errors.Join(context.Canceled, net.ErrClosed),
		net.ErrClosed,
		io.EOF,
		errors.New("stream failed"),
	} {
		t.Run(outputErr.Error(), func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			release := make(chan struct{})
			propagated := make(chan struct{})
			ctx := delayedPropagationContext{Context: parent, release: release, propagated: propagated}
			defer func() {
				close(release)
				<-propagated
			}()
			conn := &cancelingReadConn{
				scriptedConn: newScriptedConn(""),
				cancel:       cancel,
				err:          outputErr,
			}
			restored := false
			err := BridgeWithTerminal(ctx, conn, strings.NewReader(""), io.Discard,
				func(io.Reader) (func() error, error) {
					return func() error { restored = true; return nil }, nil
				})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("bridge error = %v, want caller cancellation", err)
			}
			if !conn.isClosed() || !restored {
				t.Errorf("bridge teardown: closed=%v, restored=%v", conn.isClosed(), restored)
			}
		})
	}
}

// A caller's cancellation can close its transport before NotifyContext's child
// observes it. Delay that child notification through Context's supported
// AfterFunc hook so the regression does not depend on goroutine scheduling.
type delayedPropagationContext struct {
	context.Context
	release    <-chan struct{}
	propagated chan<- struct{}
}

func (c delayedPropagationContext) Value(any) any { return nil }
func (c delayedPropagationContext) AfterFunc(f func()) func() bool {
	return context.AfterFunc(c.Context, func() {
		<-c.release
		defer close(c.propagated)
		f()
	})
}

type cancelingReadConn struct {
	*scriptedConn
	cancel context.CancelFunc
	err    error
}

func (c *cancelingReadConn) Read([]byte) (int, error) {
	c.cancel()
	return 0, c.err
}

func TestBridgeCallerCancellationDuringRestore(t *testing.T) {
	for _, cancelOnRestore := range []bool{false, true} {
		name := "restore failure"
		if cancelOnRestore {
			name = "cancellation wins over restore failure"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			restoreErr := errors.New("restore failed")
			conn := newScriptedConn("")
			err := BridgeWithTerminal(ctx, conn, strings.NewReader(""), io.Discard,
				func(io.Reader) (func() error, error) {
					return func() error {
						if cancelOnRestore {
							cancel()
						}
						return restoreErr
					}, nil
				})
			want := restoreErr
			if cancelOnRestore {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Fatalf("bridge error = %v, want %v", err, want)
			}
		})
	}
}
