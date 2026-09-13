// Package streamio owns transport-neutral, bounded TCP forwarding mechanics.
package streamio

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
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
	if first != nil {
		closeBoth()
	}
	return errors.Join(first, <-done, ctx.Err())
}

// Serve accepts only loopback TCP listeners. Each connection opens a fresh
// upstream; at most 16 connections are active, and return means all have closed.
func Serve(ctx context.Context, listener net.Listener, open func(context.Context) (net.Conn, error), observe func(error)) error {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() || address.Zone != "" || open == nil {
		return errors.New("loopback TCP listener required")
	}
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer func() { cancel(); _ = listener.Close(); workers.Wait(); stop() }()
	slots := make(chan struct{}, 16)
	for {
		local, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			_ = local.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots; _ = local.Close() }()
			stopLocal := context.AfterFunc(ctx, func() { _ = local.Close() })
			defer stopLocal()
			upstream, err := open(ctx)
			if err == nil {
				err = Relay(ctx, local, upstream)
				if completion, ok := upstream.(interface{ Wait(context.Context) error }); ok && ctx.Err() == nil {
					waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					err = errors.Join(err, completion.Wait(waitCtx))
					cancel()
				}
			}
			if observe != nil {
				observe(err)
			}
		}()
	}
}
