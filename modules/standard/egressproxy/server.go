package egressproxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

// Serve owns the listener and every accepted connection, including hijacked
// CONNECT tunnels. Stopping this service never leaves an authorized tunnel
// alive outside its controller lifetime.
func (p *Proxy) Serve(ctx context.Context, listener net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	connections := &proxyListener{Listener: listener, connections: make(map[*proxyConnection]struct{})}
	defer connections.Close()
	server := &http.Server{
		Handler:           p,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// net/http may include arbitrary panic values and stacks in ErrorLog.
		// Report only the selected failure boundary through the shared logger.
		ErrorLog: log.New(proxyErrorWriter{ctx: ctx}, "", 0),
	}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		_ = connections.Close()
		_ = server.Close()
		close(stopped)
	}()
	err := server.Serve(connections)
	contextErr := ctx.Err()
	cancel()
	<-stopped
	if contextErr != nil && (errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed)) {
		return contextErr
	}
	return err
}

type proxyErrorWriter struct{ ctx context.Context }

func (w proxyErrorWriter) Write(data []byte) (int, error) {
	logging.FromContext(w.ctx).ErrorContext(w.ctx, "proxy HTTP transport failed", "component", "proxy", "operation", "serve_http")
	return len(data), nil
}

type proxyListener struct {
	net.Listener
	mu          sync.Mutex
	closed      bool
	connections map[*proxyConnection]struct{}
}

func (l *proxyListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			_ = conn.Close()
			return nil, net.ErrClosed
		}
		if len(l.connections) >= 256 {
			l.mu.Unlock()
			_ = conn.Close()
			continue
		}
		tracked := &proxyConnection{Conn: conn, owner: l}
		l.connections[tracked] = struct{}{}
		l.mu.Unlock()
		return tracked, nil
	}
}

func (l *proxyListener) Close() error {
	l.mu.Lock()
	l.closed = true
	connections := make([]*proxyConnection, 0, len(l.connections))
	for conn := range l.connections {
		connections = append(connections, conn)
	}
	l.mu.Unlock()
	err := l.Listener.Close()
	for _, conn := range connections {
		_ = conn.Close()
	}
	return err
}

type proxyConnection struct {
	net.Conn
	owner *proxyListener
}

func (c *proxyConnection) Close() error {
	err := c.Conn.Close()
	c.owner.mu.Lock()
	delete(c.owner.connections, c)
	c.owner.mu.Unlock()
	return err
}

func (c *proxyConnection) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}
