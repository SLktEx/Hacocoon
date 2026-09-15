//go:build linux

package control

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixDialerPreservesCancellationAndDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.sock")
	listener, err := ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		want := context.Canceled
		if expired {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			want = context.DeadlineExceeded
		}
		conn, err := UnixDialer(path)(ctx)
		if conn != nil {
			_ = conn.Close()
			t.Fatal("cancelled dial opened a connection")
		}
		if !errors.Is(err, want) || errors.Is(err, ErrUnavailable) {
			t.Fatal("caller cancellation was classified as unavailable", err)
		}
	}
	// Refused/missing endpoints retain their independent unavailable category.
	if conn, err := UnixDialer(filepath.Join(t.TempDir(), "missing.sock"))(context.Background()); conn != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatal(conn, err)
	}
	if conn, err := UnixDialer(" ")(context.Background()); conn != nil || !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(conn, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := UnixDialer(path)(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, ok := conn.(*net.UnixConn); !ok {
		t.Fatal("ordinary Unix connection changed")
	}
}
