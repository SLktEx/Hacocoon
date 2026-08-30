package control

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
)

func TestStreamCloseWriteKeepsReadSideOpen(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.RegisterStream("half-close", func(_ context.Context, _ json.RawMessage) (Stream, error) {
			return func(_ context.Context, conn net.Conn) error {
				payload, err := io.ReadAll(conn)
				if err != nil {
					return err
				}
				_, err = conn.Write(append([]byte("ack:"), payload...))
				return err
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()

	stream, err := client.OpenStream(context.Background(), "half-close", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	closer, ok := stream.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("stream does not expose CloseWrite")
	}
	if err := closer.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != "ack:hello" {
		t.Fatalf("response = %q, want ack:hello", response)
	}
}
