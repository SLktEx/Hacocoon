package control

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestTerminalControlNegotiationAndByteIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewServer()
	updates := make(chan [2]int, 4)
	if err := server.RegisterStream("shell", func(ctx context.Context, _ json.RawMessage) (Stream, error) {
		SetTerminalResizeHandler(ctx, func(columns, rows int) error {
			updates <- [2]int{columns, rows}
			return nil
		})
		return func(_ context.Context, conn net.Conn) error {
			data := make([]byte, 6)
			if _, err := io.ReadFull(conn, data); err != nil {
				return err
			}
			_, err := conn.Write(data)
			return err
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	client, _ := NewClient(func(context.Context) (net.Conn, error) {
		local, remote := net.Pipe()
		go server.serveConn(ctx, remote)
		return local, nil
	})
	conn, err := client.OpenSession(ctx, "shell", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	session := conn.(*sessionConn)
	if !session.SupportsResize() {
		t.Fatal("resize capability was not negotiated")
	}
	for _, size := range [][2]int{{132, 43}, {37, 17}} {
		if err := session.Resize(ctx, size[0], size[1]); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-updates:
			if got != size {
				t.Fatalf("size = %v, want %v", got, size)
			}
		case <-ctx.Done():
			t.Fatal("resize was not delivered")
		}
	}
	for _, size := range [][2]int{{0, 10}, {-1, 10}, {10001, 10}, {10, 0}} {
		if session.Resize(ctx, size[0], size[1]) == nil {
			t.Fatalf("accepted invalid size %v", size)
		}
	}
	if err := client.Call(ctx, methodSessionResize, sessionResizeRequest{strings.Repeat("f", 32), 80, 24}, nil); err == nil {
		t.Fatal("accepted unknown session identity")
	}
	// Include NUL and escape bytes: none can be interpreted as resize controls.
	want := []byte{'a', 0, 27, '[', 'D', '\n'}
	if _, err := conn.Write(want); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(conn)
	if err != nil || string(got) != string(want) {
		t.Fatalf("stream = %q, %v", got, err)
	}
	if session.Resize(ctx, 80, 24) == nil {
		t.Fatal("accepted resize after session completion")
	}
	select {
	case size := <-updates:
		t.Fatalf("invalid request reached handler: %v", size)
	default:
	}
}

func TestTerminalControlLegacyAndNonTerminalSessions(t *testing.T) {
	for _, managed := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		server := NewServer()
		if err := server.RegisterStream("bytes", func(context.Context, json.RawMessage) (Stream, error) {
			return func(_ context.Context, conn net.Conn) error {
				_, err := io.WriteString(conn, "plain bytes")
				return err
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
		client, _ := NewClient(func(context.Context) (net.Conn, error) {
			local, remote := net.Pipe()
			go server.serveConn(ctx, remote)
			return local, nil
		})
		open := client.OpenStream
		if managed {
			open = client.OpenSession
		}
		conn, err := open(ctx, "bytes", nil)
		if err != nil {
			t.Fatal(err)
		}
		if session, ok := conn.(*sessionConn); ok && session.SupportsResize() {
			t.Fatal("non-terminal session advertised resize")
		}
		data, err := io.ReadAll(conn)
		conn.Close()
		cancel()
		if err != nil || string(data) != "plain bytes" {
			t.Fatalf("legacy/non-terminal output = %q, %v", data, err)
		}
	}
}
