package controlapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/streamio"
	"io"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type streamForwardFixture struct {
	address string
	failure error
	active  atomic.Int32
}

func (f *streamForwardFixture) PrepareTCPForward(_ context.Context, name, address string, port int) (core.EnvironmentTCPForward, error) {
	return core.EnvironmentTCPForward{Environment: name, Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: address, Port: port}, nil
}
func (f *streamForwardFixture) DialTCPForward(ctx context.Context, _ core.EnvironmentTCPForward) (net.Conn, error) {
	if f.failure != nil {
		return nil, f.failure
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", f.address)
	if err != nil {
		return nil, err
	}
	f.active.Add(1)
	return &countedForwardSocket{Conn: conn, active: &f.active}, nil
}

type countedForwardSocket struct {
	net.Conn
	active *atomic.Int32
	once   sync.Once
}

func (c *countedForwardSocket) Close() error {
	c.once.Do(func() { c.active.Add(-1) })
	return c.Conn.Close()
}
func (c *countedForwardSocket) CloseWrite() error { return c.Conn.(*net.TCPConn).CloseWrite() }

func startForwardController(t *testing.T, f *streamForwardFixture) (*Client, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	server := control.NewServer()
	if err := RegisterForwardStreams(server, f); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	client, err := NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	return client, ctx
}

func TestForwardStreamConcurrentBinaryHalfCloseAndCleanup(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	fixture := &streamForwardFixture{address: target.Addr().String()}
	client, ctx := startForwardController(t, fixture)
	payload := bytes.Repeat([]byte{0, 1, 10, 13, 255}, 40000)
	var remote sync.WaitGroup
	remote.Add(8)
	go func() {
		for i := 0; i < 8; i++ {
			conn, e := target.Accept()
			if e != nil {
				return
			}
			go func() {
				defer remote.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				data, e := io.ReadAll(conn)
				if e == nil {
					_, _ = conn.Write(data)
				}
			}()
		}
	}()
	selected, err := client.PrepareEnvironmentForward(ctx, "demo", "127.0.0.1", 8080)
	if err != nil {
		t.Fatal(err)
	}
	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	observed := make(chan error, 8)
	served := make(chan error, 1)
	go func() {
		served <- streamio.Serve(runCtx, local, func(ctx context.Context) (net.Conn, error) { return client.OpenEnvironmentForward(ctx, selected) }, func(e error) { observed <- e })
	}()
	var users sync.WaitGroup
	for i := 0; i < 8; i++ {
		users.Add(1)
		go func() {
			defer users.Done()
			conn, e := net.Dial("tcp", local.Addr().String())
			if e != nil {
				t.Error(e)
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			if _, e = conn.Write(payload); e != nil {
				t.Error(e)
				return
			}
			_ = conn.(*net.TCPConn).CloseWrite()
			data, e := io.ReadAll(conn)
			if e != nil || !bytes.Equal(data, payload) {
				t.Errorf("binary round trip: bytes=%d error=%v", len(data), e)
			}
		}()
	}
	users.Wait()
	remote.Wait()
	for i := 0; i < 8; i++ {
		select {
		case e := <-observed:
			if e != nil {
				t.Fatal(e)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	cancel()
	if e := <-served; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if fixture.active.Load() != 0 {
		t.Fatal("provider socket leaked", fixture.active.Load())
	}
	if conn, e := net.DialTimeout("tcp", local.Addr().String(), time.Second); e == nil {
		conn.Close()
		t.Fatal("listener leaked")
	}
}

func TestForwardStreamRefusalIsNotSuccessfulEOF(t *testing.T) {
	for _, failure := range []error{core.ErrPolicyDenied, core.ErrCapabilityStale, core.ErrNotFound, core.ErrIncompatibleState} {
		t.Run(failure.Error(), func(t *testing.T) {
			f := &streamForwardFixture{failure: failure}
			client, ctx := startForwardController(t, f)
			target, _ := client.PrepareEnvironmentForward(ctx, "demo", "127.0.0.1", 80)
			conn, err := client.OpenEnvironmentForward(ctx, target)
			if conn != nil || err == nil {
				t.Fatal(conn, err)
			}
			var status *control.StatusError
			if !errors.As(err, &status) || status.Code != statusFromError(failure).Code {
				t.Fatal(err)
			}
		})
	}
}

func TestForwardStreamPeerEOFDoesNotWaitForOurEOF(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	f := &streamForwardFixture{address: target.Addr().String()}
	client, ctx := startForwardController(t, f)
	received := make(chan string, 1)
	go func() {
		conn, e := target.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_ = conn.(*net.TCPConn).CloseWrite()
		b, _ := io.ReadAll(conn)
		received <- string(b)
	}()
	selected, _ := client.PrepareEnvironmentForward(ctx, "demo", "127.0.0.1", 80)
	conn, err := client.OpenEnvironmentForward(ctx, selected)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 1)
	if _, err = conn.Read(b); !errors.Is(err, io.EOF) {
		t.Fatal("peer half-close blocked behind completion", err)
	}
	_, _ = conn.Write([]byte("after EOF"))
	_ = conn.(interface{ CloseWrite() error }).CloseWrite()
	select {
	case data := <-received:
		if data != "after EOF" {
			t.Fatal(data)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = conn.(interface{ Wait(context.Context) error }).Wait(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestForwardStreamCloseCancelsPeerThatIgnoresEOF(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	f := &streamForwardFixture{address: target.Addr().String()}
	client, ctx := startForwardController(t, f)
	release := make(chan struct{})
	serverDone := make(chan struct{})
	defer func() { close(release); target.Close(); <-serverDone }()
	go func() {
		defer close(serverDone)
		conn, e := target.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
		<-release
	}()
	selected, _ := client.PrepareEnvironmentForward(ctx, "demo", "127.0.0.1", 80)
	conn, err := client.OpenEnvironmentForward(ctx, selected)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	if f.active.Load() != 0 {
		t.Fatal("cancellation acknowledged before provider socket closed", f.active.Load())
	}
}
