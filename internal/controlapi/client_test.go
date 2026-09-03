package controlapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeEnvironments struct {
	deleted string
}

func (*fakeEnvironments) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if spec.Name == "" || spec.WorkspacePath == "" {
		return core.Environment{}, core.ErrInvalidArgument
	}
	return core.Environment{
		Name:       spec.Name,
		Workspace:  core.Workspace{ID: core.WorkspaceID(spec.WorkspacePath), Path: spec.WorkspacePath},
		AccessMode: spec.AccessMode,
		Base:       nil,
		Resources:  spec.Resources,
		RuntimeRef: "haco-" + spec.Name,
	}, nil
}

func (*fakeEnvironments) List(context.Context) ([]core.Environment, error) {
	return []core.Environment{{Name: "demo", Workspace: core.Workspace{Path: "/work"}, AccessMode: core.WorkspaceReadWrite, RuntimeRef: "haco-demo"}}, nil
}

func (*fakeEnvironments) Exec(_ context.Context, name string, request core.ExecutionRequest) (core.ExecutionResult, error) {
	if name != "demo" || len(request.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if request.Argv[0] == "false" {
		return core.ExecutionResult{ExitCode: 7, Stderr: "failed\n"}, fmt.Errorf("exit status 7")
	}
	if len(request.Argv) != 2 || request.Argv[0] != "printf" || request.Argv[1] != "ok" {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	return core.ExecutionResult{ExitCode: 0, Stdout: "ok"}, nil
}

func (*fakeEnvironments) PrepareShellStream(_ context.Context, name string) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if name != "demo" {
		return nil, core.ErrNotFound
	}
	return func(_ context.Context, stdin io.Reader, stdout, _ io.Writer) error {
		buffer := make([]byte, 4)
		if _, err := io.ReadFull(stdin, buffer); err != nil {
			return err
		}
		_, err := stdout.Write(buffer)
		return err
	}, nil
}

func (f *fakeEnvironments) Delete(_ context.Context, name string) error {
	if name == "" {
		return core.ErrInvalidArgument
	}
	f.deleted = name
	return nil
}

type fakeClients struct{}

func (fakeClients) Status(_ context.Context, name string) (core.EnvironmentStatus, error) {
	if name != "demo" {
		return core.EnvironmentStatus{}, core.ErrNotFound
	}
	return core.EnvironmentStatus{
		Environment: core.Environment{Name: "demo", Workspace: core.Workspace{Path: "/work"}, AccessMode: core.WorkspaceReadWrite, RuntimeRef: "haco-demo"},
		State:       core.EnvironmentRunning,
	}, nil
}

func (fakeClients) Connections(_ context.Context, name string) ([]core.ClientConnection, error) {
	if name != "demo" {
		return nil, core.ErrNotFound
	}
	return []core.ClientConnection{{ID: "ssh-one", Kind: "ssh", Host: "127.0.0.1", Port: 2201, TargetPort: 22, User: "root"}}, nil
}

func (fakeClients) Forward(_ context.Context, name string, request core.LocalPortRequest) (core.ClientConnection, error) {
	if name != "demo" || request.HostPort != 8080 || request.TargetPort != 80 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	return core.ClientConnection{ID: "tcp-one", Kind: "tcp", Host: "127.0.0.1", Port: request.HostPort, TargetPort: request.TargetPort}, nil
}

func (fakeClients) Unforward(_ context.Context, name, connectionID string) error {
	if name != "demo" || connectionID == "" {
		return core.ErrInvalidArgument
	}
	return nil
}

func (fakeClients) SSH(_ context.Context, name string, request core.SSHAccessRequest) (core.ClientConnection, error) {
	if name != "demo" || request.PublicKey == "" || request.HostPort != 2202 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	return core.ClientConnection{ID: "ssh-two", Kind: "ssh", Host: "127.0.0.1", Port: request.HostPort, TargetPort: 22, User: "root"}, nil
}

func TestTypedClientLifecycleOverUnixSocket(t *testing.T) {
	environments := &fakeEnvironments{}
	client, cancel := startControlAPITestServer(t, environments)
	defer cancel()

	ping, err := client.Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ping.ProtocolVersion != control.ProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", ping.ProtocolVersion, control.ProtocolVersion)
	}

	created, err := client.CreateEnvironment(context.Background(), EnvironmentCreateRequest{
		Name:          "demo",
		WorkspacePath: "/work",
		AccessMode:    core.WorkspaceReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "demo" || created.RuntimeRef != "haco-demo" {
		t.Fatalf("created = %#v", created)
	}

	listed, err := client.ListEnvironments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != "demo" {
		t.Fatalf("listed = %#v", listed)
	}

	status, err := client.EnvironmentStatus(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != core.EnvironmentRunning || status.Environment.Name != "demo" {
		t.Fatalf("status = %#v", status)
	}

	connections, err := client.EnvironmentConnections(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].ID != "ssh-one" {
		t.Fatalf("connections = %#v", connections)
	}

	forwarded, err := client.ForwardEnvironment(context.Background(), "demo", core.LocalPortRequest{Protocol: "tcp", HostPort: 8080, TargetPort: 80})
	if err != nil {
		t.Fatal(err)
	}
	if forwarded.ID != "tcp-one" || forwarded.Port != 8080 || forwarded.TargetPort != 80 {
		t.Fatalf("forwarded = %#v", forwarded)
	}
	if err := client.UnforwardEnvironment(context.Background(), "demo", forwarded.ID); err != nil {
		t.Fatal(err)
	}

	ssh, err := client.PrepareEnvironmentSSH(context.Background(), "demo", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA test", HostPort: 2202})
	if err != nil {
		t.Fatal(err)
	}
	if ssh.ID != "ssh-two" || ssh.Port != 2202 || ssh.TargetPort != 22 {
		t.Fatalf("ssh = %#v", ssh)
	}
	if err := client.UnforwardEnvironment(context.Background(), "demo", ssh.ID); err != nil {
		t.Fatal(err)
	}

	result, err := client.ExecEnvironment(context.Background(), "demo", []string{"printf", "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Stdout != "ok" {
		t.Fatalf("exec result = %#v", result)
	}

	result, err = client.ExecEnvironment(context.Background(), "demo", []string{"false"})
	if err != nil {
		t.Fatalf("non-zero guest exit became controller error: %v", err)
	}
	if result.ExitCode != 7 || result.Stderr != "failed\n" {
		t.Fatalf("non-zero exec result = %#v", result)
	}

	stream, err := client.OpenEnvironmentShell(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte("ping")); err != nil {
		stream.Close()
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(stream, response); err != nil {
		stream.Close()
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if string(response) != "ping" {
		t.Fatalf("shell stream response = %q", response)
	}

	if err := client.DeleteEnvironment(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if environments.deleted != "demo" {
		t.Fatalf("deleted = %q, want demo", environments.deleted)
	}
}

func TestConnectionRequestsRejectMissingEnvironment(t *testing.T) {
	client, cancel := startControlAPITestServer(t, &fakeEnvironments{})
	defer cancel()

	for name, run := range map[string]func() error{
		"connections": func() error { _, err := client.EnvironmentConnections(context.Background(), ""); return err },
		"forward": func() error { _, err := client.ForwardEnvironment(context.Background(), "", core.LocalPortRequest{HostPort: 8080, TargetPort: 80}); return err },
		"unforward": func() error { return client.UnforwardEnvironment(context.Background(), "", "tcp-one") },
		"ssh": func() error { _, err := client.PrepareEnvironmentSSH(context.Background(), "", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA test", HostPort: 2202}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			var status *control.StatusError
			if err := run(); !errors.As(err, &status) || status.Code != "invalid_argument" {
				t.Fatalf("error = %v, want invalid_argument StatusError", err)
			}
		})
	}
}

func TestShellMissingEnvironmentFailsBeforeStreamOpens(t *testing.T) {
	client, cancel := startControlAPITestServer(t, &fakeEnvironments{})
	defer cancel()

	_, err := client.OpenEnvironmentShell(context.Background(), "missing")
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "not_found" {
		t.Fatalf("error = %v, want not_found StatusError", err)
	}
}

func startControlAPITestServer(t *testing.T, environments *fakeEnvironments) (*Client, context.CancelFunc) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := Register(server, environments, fakeClients{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not stop")
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return client, cancel
}
