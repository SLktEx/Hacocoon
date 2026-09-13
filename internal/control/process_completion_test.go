package control

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
)

func TestProcessReceiptDoesNotOverrideFailedSessionCompletion(t *testing.T) {
	server, wire := net.Pipe()
	failed := errors.New("session completion failed")
	client, err := NewProcessConn(context.Background(), completedProcessTestSession{Conn: wire, err: failed}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	go func() {
		_ = ServeProcess(context.Background(), server, func(context.Context, io.Reader, io.Writer, io.Writer) ([]byte, error) {
			return []byte("claimed receipt"), nil
		})
	}()
	if _, err := io.ReadAll(client); !errors.Is(err, failed) {
		t.Fatal("completion failure disappeared", err)
	}
	if _, err := client.Result(); err == nil {
		t.Fatal("unconfirmed receipt published")
	}
}
