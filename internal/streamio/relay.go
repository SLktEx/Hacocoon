// Package streamio owns transport-neutral, bounded TCP forwarding mechanics.
package streamio

import (
	"context"
	"errors"
	"io"
	"net"

	"time"
)

// Relay preserves request EOF while draining the response. It owns both sockets
// until both copy workers have stopped, including on cancellation and errors.
func Relay(ctx context.Context, a, b net.Conn) error {
	closeBoth := func() { _ = a.Close(); _ = b.Close() }
	stop := context.AfterFunc(ctx, closeBoth)
	defer stop()
	defer closeBoth()
	done := make(chan error, 2)
	copyTo := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err == nil {
			if half, ok := dst.(interface{ CloseWrite() error }); ok {
				err = half.CloseWrite()
			} else {
				err = errors.New("stream does not support half-close")
			}
		}
		done <- err
	}
	go copyTo(a, b)
	go copyTo(b, a)
	first := <-done
	// Bound a peer that abandons a half-closed session without cancellation.
	timer := time.AfterFunc(30*time.Second, closeBoth)
	defer timer.Stop()
	if first != nil {
		closeBoth()
	}
	return errors.Join(first, <-done, ctx.Err())
}
