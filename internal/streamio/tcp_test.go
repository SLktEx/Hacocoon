package streamio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestRelayRealTCPHalfCloseAndBinaryResponse(t *testing.T) {
	pair := func() (net.Conn, net.Conn) {
		t.Helper()
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		a, err := net.Dial("tcp", l.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		b, err := l.Accept()
		if err != nil {
			a.Close()
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Close(); b.Close() })
		return a, b
	}
	user, local := pair()
	upstream, application := pair()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Relay(ctx, local, upstream) }()
	server := make(chan error, 1)
	go func() {
		defer application.Close()
		data, err := io.ReadAll(application)
		if err == nil {
			_, err = application.Write(data)
		}
		server <- err
	}()
	payload := bytes.Repeat([]byte{0, 10, 13, 255}, 65536)
	_ = user.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := user.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := user.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(user)
	if err != nil || !bytes.Equal(payload, response) {
		t.Fatal(len(response), err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = <-server; err != nil {
		t.Fatal(err)
	}
}

func TestServeCancellationWaitsForPendingDialAndClosesAcceptedSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	dialStopped := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, func(ctx context.Context) (net.Conn, error) {
			close(entered)
			<-ctx.Done()
			close(dialStopped)
			return nil, ctx.Err()
		}, nil)
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	<-entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener workers did not stop")
	}
	select {
	case <-dialStopped:
	default:
		t.Fatal("returned before dial stopped")
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 1)
	if _, err = conn.Read(b); err == nil {
		t.Fatal("accepted connection retained")
	}
}

func TestServeRejectsNonLoopbackWithoutAccepting(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err = Serve(context.Background(), listener, func(context.Context) (net.Conn, error) { t.Error("dialed"); return nil, nil }, nil); err == nil {
		t.Fatal("wildcard accepted")
	}
}
