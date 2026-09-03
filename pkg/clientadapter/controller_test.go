package clientadapter

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeControllerClient struct {
	environment *core.Environment
	connections []core.ClientConnection
	revoked     string
}

func (f *fakeControllerClient) CreateEnvironment(_ context.Context, request controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	if f.environment != nil {
		return core.Environment{}, control.NewStatusError("already_exists", "already exists")
	}
	environment := core.Environment{
		Name:       request.Name,
		Workspace:  core.Workspace{Path: request.WorkspacePath},
		AccessMode: request.AccessMode,
		RuntimeRef: "haco-" + request.Name,
	}
	f.environment = &environment
	return environment, nil
}

func (f *fakeControllerClient) EnvironmentStatus(_ context.Context, name string) (core.EnvironmentStatus, error) {
	if f.environment == nil || f.environment.Name != name {
		return core.EnvironmentStatus{}, control.NewStatusError("not_found", "missing")
	}
	return core.EnvironmentStatus{Environment: *f.environment, State: core.EnvironmentRunning}, nil
}

func (f *fakeControllerClient) EnvironmentConnections(_ context.Context, name string) ([]core.ClientConnection, error) {
	if f.environment == nil || f.environment.Name != name {
		return nil, control.NewStatusError("not_found", "missing")
	}
	return append([]core.ClientConnection(nil), f.connections...), nil
}

func (f *fakeControllerClient) ForwardEnvironment(_ context.Context, name string, request core.LocalPortRequest) (core.ClientConnection, error) {
	if f.environment == nil || f.environment.Name != name {
		return core.ClientConnection{}, control.NewStatusError("not_found", "missing")
	}
	connection := core.ClientConnection{ID: "tcp-one", Kind: "tcp", Host: "127.0.0.1", Port: request.HostPort, TargetPort: request.TargetPort}
	f.connections = append(f.connections, connection)
	return connection, nil
}

func (f *fakeControllerClient) UnforwardEnvironment(_ context.Context, name, connectionID string) error {
	if f.environment == nil || f.environment.Name != name {
		return control.NewStatusError("not_found", "missing")
	}
	f.revoked = connectionID
	return nil
}

func (f *fakeControllerClient) PrepareEnvironmentSSH(_ context.Context, name string, request core.SSHAccessRequest) (core.ClientConnection, error) {
	if f.environment == nil || f.environment.Name != name {
		return core.ClientConnection{}, control.NewStatusError("not_found", "missing")
	}
	connection := core.ClientConnection{ID: "ssh-one", Kind: "ssh", Host: "127.0.0.1", Port: request.HostPort, TargetPort: 22, User: "root"}
	f.connections = append(f.connections, connection)
	return connection, nil
}

func (f *fakeControllerClient) DeleteEnvironment(_ context.Context, name string) error {
	if f.environment == nil || f.environment.Name != name {
		return control.NewStatusError("not_found", "missing")
	}
	f.environment = nil
	return nil
}

func TestControllerAdapterEnsureCreatesThenReuses(t *testing.T) {
	workspace := t.TempDir()
	client := &fakeControllerClient{}
	adapter := newControllerAdapter(client)

	first, created, err := adapter.Ensure(context.Background(), EnsureRequest{
		Name:          "demo",
		WorkspacePath: workspace,
		AccessMode:    ReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || first.Name != "demo" || first.SourceWorkspace != filepath.Clean(workspace) || first.State != Running {
		t.Fatalf("first ensure = %#v created=%v", first, created)
	}

	second, created, err := adapter.Ensure(context.Background(), EnsureRequest{
		Name:          "demo",
		WorkspacePath: workspace,
		AccessMode:    ReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created || second != first {
		t.Fatalf("second ensure = %#v created=%v, want reuse %#v", second, created, first)
	}
}

func TestControllerAdapterRefusesWorkspaceMismatch(t *testing.T) {
	workspace := t.TempDir()
	other := t.TempDir()
	client := &fakeControllerClient{environment: &core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{Path: workspace},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "haco-demo",
	}}
	adapter := newControllerAdapter(client)

	_, _, err := adapter.Ensure(context.Background(), EnsureRequest{Name: "demo", WorkspacePath: other, AccessMode: ReadWrite})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestControllerAdapterConnectionLifecycle(t *testing.T) {
	workspace := t.TempDir()
	client := &fakeControllerClient{environment: &core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{Path: workspace},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "haco-demo",
	}}
	adapter := newControllerAdapter(client)

	ssh, err := adapter.PrepareSSH(context.Background(), SSHRequest{
		Environment: "demo",
		PublicKey:   "ssh-ed25519 AAAA test",
		HostPort:    2222,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ssh.ID != "ssh-one" || ssh.Host != "127.0.0.1" || ssh.Port != 2222 || ssh.TargetPort != 22 {
		t.Fatalf("ssh = %#v", ssh)
	}

	connections, err := adapter.Connections(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0] != ssh {
		t.Fatalf("connections = %#v", connections)
	}
	if err := adapter.Revoke(context.Background(), "demo", ssh.ID); err != nil {
		t.Fatal(err)
	}
	if client.revoked != ssh.ID {
		t.Fatalf("revoked = %q, want %q", client.revoked, ssh.ID)
	}
}

func TestControllerAdapterRejectsNonLoopbackConnection(t *testing.T) {
	workspace := t.TempDir()
	client := &fakeControllerClient{
		environment: &core.Environment{
			Name:       "demo",
			Workspace:  core.Workspace{Path: workspace},
			AccessMode: core.WorkspaceReadWrite,
			RuntimeRef: "haco-demo",
		},
		connections: []core.ClientConnection{{ID: "tcp-one", Kind: "tcp", Host: "192.0.2.1", Port: 8080, TargetPort: 80}},
	}
	adapter := newControllerAdapter(client)
	_, err := adapter.Connections(context.Background(), "demo")
	if !errors.Is(err, ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestControllerErrorTranslation(t *testing.T) {
	for code, want := range map[string]error{
		"invalid_argument":   core.ErrInvalidArgument,
		"not_found":          core.ErrNotFound,
		"already_exists":     core.ErrAlreadyExists,
		"unsupported":        core.ErrUnsupported,
		"unavailable":        core.ErrRuntimeUnavailable,
		"busy":               core.ErrWorkspaceBusy,
		"incompatible_state": core.ErrIncompatibleState,
		"recovery_required":  core.ErrRecoveryRequired,
	} {
		t.Run(code, func(t *testing.T) {
			err := controllerError(control.NewStatusError(code, "example"))
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}
