package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestManagedSessionRefusesMissingCompletionIdentity(t *testing.T) {
	for _, kind := range []string{"process", "byte-relay"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			local, remote := net.Pipe()
			defer func() { _ = local.Close(); _ = remote.Close() }()
			peerDone := make(chan error, 1)
			go func() {
				var request requestEnvelope
				if err := json.NewDecoder(remote).Decode(&request); err != nil {
					peerDone <- err
					return
				}
				if !request.Stream || !request.Session {
					peerDone <- errors.New("client omitted managed-session negotiation")
					return
				}
				if err := writeJSONLine(remote, responseEnvelope{Version: ProtocolVersion}); err != nil {
					peerDone <- err
					return
				}
				var b [1]byte
				_, err := remote.Read(b[:])
				peerDone <- err
			}()
			client, err := NewClient(func(context.Context) (net.Conn, error) { return local, nil })
			if err != nil {
				t.Fatal(err)
			}
			open := client.OpenSession
			if kind == "byte-relay" {
				open = client.OpenByteSession
			}
			conn, err := open(ctx, "test", nil)
			if conn != nil {
				_ = conn.Close()
			}
			if conn != nil || !errors.Is(err, ErrProtocol) {
				t.Fatalf("peer without completion identity accepted: conn=%T err=%v", conn, err)
			}
			select {
			case err := <-peerDone:
				if !errors.Is(err, io.EOF) {
					t.Fatal("refused peer retained stream authority", err)
				}
			case <-ctx.Done():
				t.Fatal("refused connection remained open")
			}
		})
	}
}
