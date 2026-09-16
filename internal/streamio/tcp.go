// Package streamio owns transport-neutral, bounded TCP forwarding mechanics.
package streamio

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Serve accepts only loopback TCP listeners. Each connection opens a fresh
// upstream; at most 16 connections are active, and return means all have closed.
func Serve(ctx context.Context, listener net.Listener, open func(context.Context) (net.Conn, error), observe func(error)) error {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() || address.Zone != "" || open == nil {
		return errors.New("loopback TCP listener required")
	}
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	// A second Close may return before a concurrent first Close has finished.
	// Share its completion with cancellation so return also releases the socket.
	closeListener := sync.OnceFunc(func() { _ = listener.Close() })
	stop := context.AfterFunc(ctx, closeListener)
	defer func() { cancel(); closeListener(); workers.Wait(); stop() }()
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
			closeLocal := sync.OnceFunc(func() { _ = local.Close() })
			defer func() { <-slots; closeLocal() }()
			stopLocal := context.AfterFunc(ctx, closeLocal)
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
