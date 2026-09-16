package control

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// A peer can accept a request without sending the handshake. Cancellation at
// either I/O boundary must remain recognizable to callers, and close the pipe.
func TestClientCancellationDuringRequestAndHandshake(t *testing.T) {
	for _, stage := range []string{"request", "response"} {
		for _, deadline := range []bool{false, true} {
			for _, operation := range []string{"call", "stream", "session", "byte-session"} {
				t.Run(stage+"/"+operation+map[bool]string{false: "/cancel", true: "/deadline"}[deadline], func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					want := context.Canceled
					if deadline {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
						want = context.DeadlineExceeded
					}
					defer cancel()
					local, peer := net.Pipe()
					defer func() { _ = peer.Close() }()
					defer func() { _ = local.Close() }()
					if err := peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
						t.Fatal(err)
					}
					ready := make(chan struct{})
					client, err := NewClient(func(context.Context) (net.Conn, error) {
						if stage == "request" {
							close(ready)
						}
						return local, nil
					})
					if err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() {
						if operation == "call" {
							done <- client.Call(ctx, "operation", nil, nil)
							return
						}
						open := client.OpenStream
						switch operation {
						case "session":
							open = client.OpenSession
						case "byte-session":
							open = client.OpenByteSession
						}
						conn, err := open(ctx, "operation", nil)
						if conn != nil {
							_ = conn.Close()
						}
						done <- err
					}()
					if stage == "response" {
						if _, err := bufio.NewReader(peer).ReadBytes('\n'); err != nil {
							t.Fatal(err)
						}
						close(ready)
					}
					<-ready
					if !deadline {
						cancel()
					}
					select {
					case err := <-done:
						if !errors.Is(err, want) {
							t.Fatalf("lost caller cancellation: got %v, want %v", err, want)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("client kept waiting for peer after cancellation")
					}
					if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
						t.Fatalf("client left its connection open: %v", err)
					}
				})
			}
		}
	}
}
