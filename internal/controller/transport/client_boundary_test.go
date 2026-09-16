package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientRejectsInvalidPeerResponses(t *testing.T) {
	for _, mode := range []string{"call", "stream"} {
		for _, response := range []string{
			"{broken}\n", `{"version":999}` + "\n", `{"version":1`,
			`{"version":1,"error":{"code":"recovery_required","message":"ownership unresolved"}}` + "\n",
		} {
			t.Run(mode+"/"+response, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				peerDone := make(chan error, 1)
				client, err := NewClient(func(context.Context) (net.Conn, error) {
					local, remote := net.Pipe()
					go func() {
						defer func() { _ = remote.Close() }()
						_, err := readEnvelopeLine(bufio.NewReader(remote))
						if err == nil {
							_, err = io.WriteString(remote, response)
						}
						peerDone <- err
					}()
					return local, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if mode == "call" {
					err = client.Call(ctx, "operation", nil, nil)
				} else {
					var conn net.Conn
					conn, err = client.OpenStream(ctx, "operation", nil)
					if conn != nil {
						_ = conn.Close()
						t.Fatal("invalid response opened a data stream")
					}
				}
				if err == nil {
					t.Fatal("invalid response reported success")
				}
				if peerErr := <-peerDone; peerErr != nil {
					t.Fatal("fixture did not deliver the invalid response", peerErr)
				}
			})
		}
	}
}

func TestClientCannotSendInvalidRequests(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, request := range []any{make(chan int), strings.Repeat("x", maxControlEnvelopeBytes)} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			received := make(chan []byte, 1)
			client, err := NewClient(func(context.Context) (net.Conn, error) {
				local, remote := net.Pipe()
				go func() { defer func() { _ = remote.Close() }(); data, _ := io.ReadAll(remote); received <- data }()
				return local, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if stream {
				var conn net.Conn
				conn, err = client.OpenStream(ctx, "operation", request)
				if conn != nil {
					_ = conn.Close()
					t.Fatal("invalid request opened a stream")
				}
			} else {
				err = client.Call(ctx, "operation", request, nil)
			}
			if err == nil {
				t.Fatal("unencodable or oversized request accepted")
			}
			select {
			case data := <-received:
				if len(data) != 0 {
					t.Fatal("partial operation request emitted", string(data))
				}
			case <-ctx.Done():
				t.Fatal("failed encoding retained connection")
			}
		}
	}
}

func TestClientRejectsIncompatibleResultShape(t *testing.T) {
	client, cancel := startTestServer(t, func(server *Server) {
		if err := server.Register("operation", func(context.Context, json.RawMessage) (any, error) {
			return "unexpected string", nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer cancel()
	var result struct{ Count int }
	if err := client.Call(context.Background(), "operation", nil, &result); err == nil || result.Count != 0 {
		t.Fatal("incompatible response accepted", result, err)
	}
}

func TestProcessRequiresCompletionAndOptionalResizeCapabilities(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close(); _ = remote.Close() }()
	for _, tc := range []struct {
		ctx    context.Context
		conn   net.Conn
		stderr io.Writer
	}{
		{nil, completedProcessTestSession{Conn: local}, io.Discard},
		{context.Background(), nil, io.Discard},
		{context.Background(), completedProcessTestSession{Conn: local}, nil},
		{context.Background(), local, io.Discard},
	} {
		if conn, err := NewProcessConn(tc.ctx, tc.conn, tc.stderr); conn != nil || !errors.Is(err, ErrInvalidArgument) {
			t.Fatal("process accepted missing completion contract", err)
		}
	}
	conn, err := NewProcessConn(context.Background(), completedProcessTestSession{Conn: local}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if conn.SupportsResize() || !errors.Is(conn.Resize(context.Background(), 80, 24), ErrUnavailable) {
		t.Fatal("unnegotiated resize was accepted")
	}
}
