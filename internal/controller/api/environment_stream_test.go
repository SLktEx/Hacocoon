//go:build linux

package controlapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

type streamTestService struct {
	dial func(context.Context, core.StreamTarget) (net.Conn, error)
}

func (s streamTestService) DialStream(c context.Context, t core.StreamTarget) (net.Conn, error) {
	return s.dial(c, t)
}
func streamTestTarget() core.StreamTarget {
	return core.StreamTarget{Environment: "dev", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}
}
func streamTestServer(t *testing.T, service streamTestService) (*Client, context.CancelFunc) {
	t.Helper()
	listener, err := control.ListenUnix(filepath.Join(t.TempDir(), "control.sock"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err = RegisterEnvironmentStreams(server, service); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("controller leak")
		}
	})
	client, err := NewClient(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return client, cancel
}
func TestEnvironmentStreamRawBytesHalfCloseAndConcurrency(t *testing.T) {
	// Unix sockets exercise actual kernel EOF semantics, not net.Pipe.
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "target.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	payload := append([]byte("SSH-2.0-test\r\n"), bytes.Repeat([]byte{0, 255, 10, 13, 128}, 4096)...)
	completed := make(chan struct{}, 8)
	go func() {
		for i := 0; i < 8; i++ {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				b, _ := io.ReadAll(conn)
				if bytes.Equal(b, payload) {
					_, _ = conn.Write(b)
				}
				completed <- struct{}{}
			}()
		}
	}()
	client, _ := streamTestServer(t, streamTestService{dial: func(ctx context.Context, _ core.StreamTarget) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", listener.Addr().String())
	}})
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, e := client.OpenEnvironmentStream(ctx, streamTestTarget())
			if e != nil {
				done <- e
				return
			}
			defer c.Close()
			_, e = c.Write(payload)
			if e == nil {
				e = c.(interface{ CloseWrite() error }).CloseWrite()
			}
			if e != nil {
				done <- e
				return
			}
			b, e := io.ReadAll(c)
			if e == nil && !bytes.Equal(b, payload) {
				e = errors.New("metadata polluted raw stream")
			}
			if e == nil {
				e = c.(interface{ Wait(context.Context) error }).Wait(ctx)
			}
			done <- e
		}()
	}
	for i := 0; i < 8; i++ {
		if e := <-done; e != nil {
			t.Error(e)
		}
		<-completed
	}
}
func TestEnvironmentStreamCancelAndControllerDisconnect(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "controller-stop"}[disconnect], func(t *testing.T) {
			target, peer := net.Pipe()
			defer peer.Close()
			client, stop := streamTestServer(t, streamTestService{dial: func(context.Context, core.StreamTarget) (net.Conn, error) { return target, nil }})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, err := client.OpenEnvironmentStream(ctx, streamTestTarget())
			if err != nil {
				t.Fatal(err)
			}
			if disconnect {
				stop()
			} else {
				if err = conn.Close(); err != nil {
					t.Fatal(err)
				}
			}
			_ = peer.SetReadDeadline(time.Now().Add(time.Second))
			var b [1]byte
			_, err = peer.Read(b[:])
			if !errors.Is(err, io.EOF) {
				t.Fatalf("target retained after disconnect: %v", err)
			}
			_ = conn.Close()
		})
	}
}
func TestEnvironmentStreamTargetFailureHasNoApplicationBytes(t *testing.T) {
	client, _ := streamTestServer(t, streamTestService{dial: func(context.Context, core.StreamTarget) (net.Conn, error) { return nil, core.ErrPolicyDenied }})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := client.OpenEnvironmentStream(ctx, streamTestTarget())
	if c != nil || err == nil {
		t.Fatalf("%v %v", c, err)
	}
}
