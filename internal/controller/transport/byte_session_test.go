package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestByteSessionEOFDoesNotWaitForOperationCompletion(t *testing.T) {
	for _, exitCode := range []int{0, 17} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		finish := make(chan struct{})
		finishOnce := sync.OnceFunc(func() { close(finish) })
		t.Cleanup(finishOnce)
		client, stop := startTestServer(t, func(server *Server) {
			if err := server.RegisterStream("bytes", func(context.Context, json.RawMessage) (Stream, error) {
				return func(ctx context.Context, conn net.Conn) error {
					input, err := io.ReadAll(conn)
					if err != nil {
						return err
					}
					if _, err := conn.Write(append([]byte("response:"), input...)); err != nil {
						return err
					}
					if err := conn.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
						return err
					}
					select {
					case <-finish:
					case <-ctx.Done():
						return ctx.Err()
					}
					if exitCode != 0 {
						return testSessionExitError{code: exitCode}
					}
					return nil
				}, nil
			}); err != nil {
				t.Fatal(err)
			}
		})
		stream, err := client.OpenByteSession(ctx, "bytes", nil)
		if err != nil {
			cancel()
			stop()
			t.Fatal(err)
		}
		t.Cleanup(func() { finishOnce(); _ = stream.Close(); cancel(); stop() })
		if _, err := stream.Write([]byte("input\x00\xff")); err != nil {
			t.Fatal(err)
		}
		if err := stream.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
			t.Fatal(err)
		}
		output, err := io.ReadAll(stream)
		if err != nil || string(output) != "response:input\x00\xff" {
			t.Fatal("byte EOF waited for completion or changed data", string(output), err)
		}
		finishOnce()
		err = stream.(interface{ Wait(context.Context) error }).Wait(ctx)
		if exitCode == 0 {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			var exit *SessionExitError
			if !errors.As(err, &exit) || exit.ExitCode() != exitCode || exit.Error() != "remote session exited 17" {
				t.Fatal("process status lost after byte EOF", err)
			}
		}
		// Wait consumes the one-shot completion. An unnecessary cancel here
		// would fail with not_found; both EOFs must make Close local only.
		if err := stream.Close(); err != nil {
			t.Fatal("completed byte session sent cancellation", err)
		}
		cancel()
		stop()
	}
}

func TestByteSessionCloseWaitsForRemoteCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cancelled, release := make(chan struct{}), make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
	client, stop := startTestServer(t, func(server *Server) {
		if err := server.RegisterStream("bytes", func(context.Context, json.RawMessage) (Stream, error) {
			return func(ctx context.Context, _ net.Conn) error {
				<-ctx.Done()
				close(cancelled)
				<-release
				return ctx.Err()
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer stop()
	stream, err := client.OpenByteSession(ctx, "bytes", nil)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- stream.Close() }()
	select {
	case <-cancelled:
	case <-ctx.Done():
		t.Fatal("Close did not cancel the remote operation")
	}
	select {
	case err := <-closed:
		t.Fatal("Close returned before remote cleanup", err)
	case <-time.After(25 * time.Millisecond):
	}
	releaseOnce()
	if err := <-closed; err != nil {
		t.Fatal("cleanup completion was lost", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal("second Close repeated cancellation", err)
	}
}
