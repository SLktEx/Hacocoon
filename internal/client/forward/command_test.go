package clientforward

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type forwardFixture struct{ address string }

func (f forwardFixture) PrepareTCPForward(_ context.Context, name, address string, port int) (core.EnvironmentTCPForward, error) {
	return core.EnvironmentTCPForward{Environment: name, Instance: "env-00000000000000000000000000000001", Address: address, Port: port}, nil
}
func (f forwardFixture) DialTCPForward(ctx context.Context, _ core.EnvironmentTCPForward) (net.Conn, error) {
	var dial net.Dialer
	return dial.DialContext(ctx, "tcp", f.address)
}

type readyOutput chan string

func (w readyOutput) Write(p []byte) (int, error) { w <- string(p); return len(p), nil }

func TestClientListenerThroughControllerConcurrentBinaryAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	application, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	var applications sync.WaitGroup
	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		for range 8 {
			c, err := application.Accept()
			if err != nil {
				return
			}
			applications.Go(func() {
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(8 * time.Second))
				data, err := io.ReadAll(c)
				if err == nil {
					_, _ = c.Write(data)
					_ = c.(*net.TCPConn).CloseWrite()
				}
			})
		}
	}()
	defer func() { _ = application.Close(); <-appDone; applications.Wait() }()
	management, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err = controlapi.RegisterForwardStreams(server, forwardFixture{application.Addr().String()}); err != nil {
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(ctx, management) }()
	defer func() { cancel(); <-serverDone }()
	connect := func() (*controlapi.Client, error) {
		return controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", management.Addr().String())
		})
	}
	ready := make(readyOutput, 1)
	var diagnostic bytes.Buffer
	commandDone := make(chan int, 1)
	go func() {
		commandDone <- Command(ctx, []string{"--target-port", "8080", "demo"}, ready, &diagnostic, cliui.English, connect, nil)
	}()
	var text string
	select {
	case text = <-ready:
	case <-ctx.Done():
		t.Fatal("no listener", ctx.Err())
	}
	match := regexp.MustCompile(`Listening at (127\.0\.0\.1:[0-9]+) → demo `).FindStringSubmatch(text)
	if len(match) != 2 {
		cancel()
		<-commandDone
		t.Fatal(text)
	}
	data := bytes.Repeat([]byte{0, 13, 10, 255}, 256<<10)
	outcomes := make(chan error, 8)
	for range 8 {
		go func() {
			c, err := net.DialTimeout("tcp", match[1], time.Second)
			if err != nil {
				outcomes <- err
				return
			}
			defer func() { _ = c.Close() }()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err = c.Write(data); err == nil {
				err = c.(*net.TCPConn).CloseWrite()
			}
			if err == nil {
				var got []byte
				got, err = io.ReadAll(c)
				if err == nil && !bytes.Equal(got, data) {
					err = errors.New("binary response differs")
				}
			}
			outcomes <- err
		}()
	}
	for range 8 {
		if err = <-outcomes; err != nil {
			cancel()
			<-commandDone
			t.Fatal(err)
		}
	}
	cancel()
	if code := <-commandDone; code != 0 {
		t.Fatalf("exit %d: %s", code, diagnostic.String())
	}
	c, err := net.DialTimeout("tcp", match[1], 100*time.Millisecond)
	if err == nil {
		_ = c.Close()
		t.Fatal("listener survived cancellation")
	}
}

func TestInvalidTargetNeverCreatesControllerClient(t *testing.T) {
	for _, args := range [][]string{{"--target-port", "0", "demo"}, {"--target-port", "80", "--listen", "0.0.0.0:0", "demo"}, {"--target-port", "80", "--address", "192.0.2.1", "demo"}} {
		var out bytes.Buffer
		connect := func() (*controlapi.Client, error) { t.Fatal("invalid input reached controller"); return nil, nil }
		if code := Command(context.Background(), args, &out, &out, cliui.English, connect, nil); code != 2 {
			t.Fatal(args, code)
		}
	}
}
