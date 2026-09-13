package streamio

import (
	"context"
	"io"
	"net"
	"time"
)

// BridgeStdio adapts process framing to one existing controller connection.
// It owns the supplied pipes; cancellation also interrupts a pending dial.
func BridgeStdio(ctx context.Context, input io.ReadCloser, output io.WriteCloser, dial func(context.Context) (net.Conn, error)) error {
	framed := NewFramedConn(input, output)
	defer framed.Close()
	stop := context.AfterFunc(ctx, func() { _ = framed.Close() })
	defer stop()
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	upstream, err := dial(dialCtx)
	cancel()
	if err != nil {
		return err
	}
	return Relay(ctx, framed, upstream)
}
