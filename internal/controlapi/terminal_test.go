package controlapi

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Exercise the real envelopes and API with a transport-neutral listener; this
// also runs in restricted test containers that cannot create Unix sockets.
type terminalTestListener struct {
	connections chan net.Conn
	done        chan struct{}
	once        sync.Once
}

func (l *terminalTestListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}
func (l *terminalTestListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *terminalTestListener) Addr() net.Addr { return &net.UnixAddr{Name: "memory", Net: "unix"} }

func TestShellTerminalControlsReachBothServices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := control.NewServer()
	hosts, environments := &fakeHostService{}, &fakeEnvironments{}
	if err := RegisterHost(server, hosts); err != nil {
		t.Fatal(err)
	}
	if err := Register(server, environments, fakeClients{}); err != nil {
		t.Fatal(err)
	}
	listener := &terminalTestListener{connections: make(chan net.Conn), done: make(chan struct{})}
	defer listener.Close()
	go server.Serve(ctx, listener)
	wire, _ := control.NewClient(func(ctx context.Context) (net.Conn, error) {
		local, remote := net.Pipe()
		select {
		case listener.connections <- remote:
			return local, nil
		case <-ctx.Done():
			local.Close()
			remote.Close()
			return nil, ctx.Err()
		}
	})
	metadata := TerminalMetadata{Term: "xterm-256color", Columns: 132, Rows: 43}
	for _, method := range []string{MethodHostShell, MethodEnvironmentShell} {
		t.Run(method, func(t *testing.T) {
			var request any = HostShellRequest{Terminal: metadata, DisplayLanguage: "ja"}
			if method == MethodEnvironmentShell {
				request = EnvironmentShellRequest{Environment: "demo", Terminal: metadata}
			}
			conn, err := wire.OpenSession(ctx, method, request)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			got := hosts.terminalMetadata
			if method == MethodEnvironmentShell {
				got = environments.shellMetadata
			}
			wantLanguage := "ja"
			if method == MethodEnvironmentShell {
				wantLanguage = ""
			}
			if got.DisplayLanguage != wantLanguage {
				t.Fatalf("language crossed shell boundary: %q", got.DisplayLanguage)
			}
			if got.Columns != 132 || got.Rows != 43 || got.Resizes == nil {
				t.Fatalf("terminal metadata = %#v", got)
			}
			resizer := conn.(interface {
				Resize(context.Context, int, int) error
			})
			for _, width := range []int{80, 90, 100} {
				if err := resizer.Resize(ctx, width, 24); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case size := <-got.Resizes:
				if size != (core.TerminalSize{Columns: 100, Rows: 24}) {
					t.Fatalf("coalesced size = %v", size)
				}
			case <-ctx.Done():
				t.Fatal("resize did not reach service")
			}
			if _, err := io.WriteString(conn, "data"); err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(conn)
			if err != nil || string(data) != "data" {
				t.Fatalf("shell data = %q, %v", data, err)
			}
		})
	}
}

func TestShellTerminalDimensionValidation(t *testing.T) {
	for _, size := range [][2]int{{0, 24}, {80, 0}, {-1, 24}, {10001, 24}} {
		if _, err := validateTerminalMetadata(TerminalMetadata{Columns: size[0], Rows: size[1]}); err == nil {
			t.Fatalf("accepted %v", size)
		}
	}
	if _, err := validateTerminalMetadata(TerminalMetadata{}); err != nil {
		t.Fatalf("non-TTY metadata: %v", err)
	}
}
