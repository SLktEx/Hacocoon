package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCallOverUnixSocket(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.Register("echo", func(_ context.Context, payload json.RawMessage) (any, error) {
			var request struct{ Value string `json:"value"` }
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return request, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()

	var response struct{ Value string `json:"value"` }
	if err := client.Call(context.Background(), "echo", map[string]string{"value": "hello"}, &response); err != nil {
		t.Fatal(err)
	}
	if response.Value != "hello" {
		t.Fatalf("value = %q, want hello", response.Value)
	}
}

func TestOpenStreamPreservesBufferedBytes(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.RegisterStream("upper", func(_ context.Context, _ json.RawMessage) (Stream, error) {
			return func(_ context.Context, conn net.Conn) error {
				buffer := make([]byte, 5)
				if _, err := io.ReadFull(conn, buffer); err != nil {
					return err
				}
				_, err := conn.Write([]byte(strings.ToUpper(string(buffer))))
				return err
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()

	stream, err := client.OpenStream(context.Background(), "upper", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(stream, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "HELLO" {
		t.Fatalf("stream response = %q, want HELLO", response)
	}
}

func TestStreamValidationErrorIsReturnedBeforeAck(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.RegisterStream("denied", func(context.Context, json.RawMessage) (Stream, error) {
			return nil, NewStatusError("denied", "stream refused")
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()

	_, err := client.OpenStream(context.Background(), "denied", nil)
	var status *StatusError
	if !errors.As(err, &status) || status.Code != "denied" {
		t.Fatalf("error = %v, want denied StatusError", err)
	}
}

func TestOpenStreamClosesOnContextCancellation(t *testing.T) {
	client, cancelServer := startTestServer(t, func(server *Server) {
		if err := server.RegisterStream("wait", func(_ context.Context, _ json.RawMessage) (Stream, error) {
			return func(_ context.Context, conn net.Conn) error {
				buffer := make([]byte, 1)
				_, err := conn.Read(buffer)
				return err
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancelServer()

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.OpenStream(ctx, "wait", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := stream.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 1)
	if _, err := stream.Read(buffer); err == nil {
		t.Fatal("stream remained open after context cancellation")
	}
}

func TestStatusErrorRoundTrip(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.Register("fail", func(context.Context, json.RawMessage) (any, error) {
			return nil, NewStatusError("denied", "not allowed")
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()

	err := client.Call(context.Background(), "fail", nil, nil)
	var status *StatusError
	if !errors.As(err, &status) {
		t.Fatalf("error = %v, want StatusError", err)
	}
	if status.Code != "denied" || status.Message != "not allowed" {
		t.Fatalf("status = %#v", status)
	}
}

func TestListenUnixReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	address := &net.UnixAddr{Name: path, Net: "unix"}
	stale, err := net.ListenUnix("unix", address)
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
}

func TestListenUnixRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	if err := os.WriteFile(path, []byte("do not delete"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ListenUnix(path, 0o600)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("error = %v, want ErrAlreadyRunning", err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil || string(content) != "do not delete" {
		t.Fatalf("regular file was modified: content=%q err=%v", content, readErr)
	}
}

func startTestServer(t *testing.T, register func(*Server)) (*Client, context.CancelFunc) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer()
	register(server)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	client, err := NewClient(UnixDialer(path))
	if err != nil {
		t.Fatal(err)
	}
	return client, cancel
}
