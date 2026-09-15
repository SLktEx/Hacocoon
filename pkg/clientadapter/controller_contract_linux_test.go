//go:build linux

package clientadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func adapterControllerSocket(t *testing.T, handlers map[string]control.Handler) string {
	t.Helper()
	server := control.NewServer()
	for method, handler := range handlers {
		if err := server.Register(method, handler); err != nil {
			t.Fatal(err)
		}
	}
	socket := filepath.Join(t.TempDir(), "adapter.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	return socket
}

func TestPublicControllerAdapterOwnsLoopbackConnectionThroughRestartAndRevoke(t *testing.T) {
	// The controller fixture opens a real loopback application socket. This is a
	// client protocol test; it does not claim Incus or Environment acceptance.
	var mu sync.Mutex
	var application net.Listener
	var connection core.ClientConnection
	deleted := false
	served := make(chan error, 1)
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if application != nil {
			_ = application.Close()
		}
	})
	workspace := t.TempDir()
	socket := adapterControllerSocket(t, map[string]control.Handler{
		controlapi.MethodEnvironmentStatus: func(_ context.Context, raw json.RawMessage) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			var request controlapi.EnvironmentNameRequest
			if err := json.Unmarshal(raw, &request); err != nil || request.Environment != "demo" {
				return nil, control.ErrInvalidArgument
			}
			if deleted {
				return nil, control.NewStatusError("not_found", "Environment removed")
			}
			return core.EnvironmentStatus{Environment: core.Environment{Name: "demo", Workspace: core.Workspace{Path: workspace}, AccessMode: core.WorkspaceReadWrite}, State: core.EnvironmentRunning}, nil
		},
		controlapi.MethodEnvironmentForward: func(_ context.Context, raw json.RawMessage) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			var request controlapi.EnvironmentForwardRequest
			if err := json.Unmarshal(raw, &request); err != nil || request.Environment != "demo" || request.Protocol != "tcp" || request.HostPort < 1 || request.TargetPort != 3000 {
				return nil, control.ErrInvalidArgument
			}
			var err error
			application, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(request.HostPort)))
			if err != nil {
				return nil, err
			}
			listener := application
			go func() {
				client, err := listener.Accept()
				if err != nil {
					served <- err
					return
				}
				defer func() { _ = client.Close() }()
				_ = client.SetDeadline(time.Now().Add(5 * time.Second))
				data := make([]byte, 4)
				if _, err := io.ReadFull(client, data); err != nil {
					served <- err
					return
				}
				_, err = client.Write(data)
				served <- err
			}()
			connection = core.ClientConnection{ID: "tcp-one", Kind: "tcp", Host: "127.0.0.1", Port: request.HostPort, TargetPort: request.TargetPort}
			return connection, nil
		},
		controlapi.MethodEnvironmentConnections: func(context.Context, json.RawMessage) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			if application == nil {
				return []core.ClientConnection{}, nil
			}
			return []core.ClientConnection{connection}, nil
		},
		controlapi.MethodEnvironmentUnforward: func(_ context.Context, raw json.RawMessage) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			var request controlapi.EnvironmentConnectionRequest
			if err := json.Unmarshal(raw, &request); err != nil || request.Environment != "demo" || request.ConnectionID != connection.ID || application == nil {
				return nil, control.ErrInvalidArgument
			}
			err := application.Close()
			application = nil
			return nil, err
		},
		controlapi.MethodEnvironmentDelete: func(_ context.Context, raw json.RawMessage) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			var request controlapi.EnvironmentNameRequest
			if err := json.Unmarshal(raw, &request); err != nil || request.Environment != "demo" {
				return nil, control.ErrInvalidArgument
			}
			deleted = true
			return nil, nil
		},
	})
	first, err := NewController()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := first.Status(ctx, "demo")
	if err != nil || status.Name != "demo" || status.SourceWorkspace != workspace || status.State != Running {
		t.Fatal(status, err)
	}
	forward, err := first.Forward(ctx, ForwardRequest{Environment: "demo", TargetPort: 3000})
	if err != nil || forward.Host != "127.0.0.1" || forward.Port < 1 {
		t.Fatal(forward, err)
	}
	address := net.JoinHostPort(forward.Host, strconv.Itoa(forward.Port))
	applicationClient, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = applicationClient.Close() }()
	_ = applicationClient.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := applicationClient.Write([]byte{0, 255, 13, 10}); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(applicationClient)
	if err != nil || string(data) != string([]byte{0, 255, 13, 10}) || <-served != nil {
		t.Fatal(data, err)
	}
	// A fresh adapter must discover the existing connection, without a client cache.
	restarted, err := NewControllerAt(socket)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := restarted.Connections(ctx, "demo")
	if err != nil || len(connections) != 1 || connections[0] != forward {
		t.Fatal(connections, err)
	}
	if err := restarted.Revoke(ctx, "demo", forward.ID); err != nil {
		t.Fatal(err)
	}
	if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("revoked listener survived")
	}
	connections, err = first.Connections(ctx, "demo")
	if err != nil || len(connections) != 0 {
		t.Fatal(connections, err)
	}
	if err := first.Delete(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Status(ctx, "demo"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := first.InteractionBatch(ctx, 0, 10); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("controller constructor unexpectedly exposed local events", err)
	}
}

func TestControllerErrorsReachPublicSentinelsWithoutReclassification(t *testing.T) {
	codes := map[string]error{"invalid_argument": ErrInvalidArgument, "not_found": ErrNotFound, "already_exists": ErrAlreadyExists, "unsupported": ErrUnsupported, "unavailable": ErrUnavailable, "busy": ErrBusy, "incompatible_state": ErrIncompatibleState, "recovery_required": ErrRecoveryRequired}
	socket := adapterControllerSocket(t, map[string]control.Handler{
		controlapi.MethodEnvironmentStatus: func(_ context.Context, raw json.RawMessage) (any, error) {
			var request controlapi.EnvironmentNameRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			return nil, control.NewStatusError(request.Environment, "operation rejected")
		},
	})
	adapter, err := NewControllerAt(socket)
	if err != nil {
		t.Fatal(err)
	}
	for code, want := range codes {
		if _, err := adapter.Status(context.Background(), code); !errors.Is(err, want) {
			t.Fatal(code, err)
		}
	}
	_, err = adapter.Status(context.Background(), "future_code")
	var remote *control.StatusError
	if !errors.As(err, &remote) || remote.Code != "future_code" {
		t.Fatal("unknown controller error lost", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.Status(ctx, "demo"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := adapter.Status(ctx, "demo"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	missing, err := NewControllerAt(filepath.Join(t.TempDir(), "absent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Status(context.Background(), "demo"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, source := range []error{core.ErrStorageBusy, core.ErrStorageUnavailable} {
		want := ErrBusy
		if source == core.ErrStorageUnavailable {
			want = ErrUnavailable
		}
		if err := translateError(fmt.Errorf("operation: %w", source)); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}

func TestControllerConstructorRejectsBlankEndpointWithPublicError(t *testing.T) {
	if adapter, err := NewControllerAt(" "); adapter != nil || !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(adapter, err)
	}
}
