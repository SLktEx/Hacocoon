package clientadapter

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

type fakeEnvironmentService struct {
	environments map[string]core.Environment
	creates      int
	deletes      []string
	createErr    error
	deleteErr    error
}

func newFakeEnvironmentService() *fakeEnvironmentService {
	return &fakeEnvironmentService{environments: map[string]core.Environment{}}
}

func (f *fakeEnvironmentService) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if f.createErr != nil {
		return core.Environment{}, f.createErr
	}
	f.creates++
	environment := core.Environment{
		Name: spec.Name,
		Workspace: core.Workspace{
			ID:   core.WorkspaceID("workspace:" + spec.Name),
			Path: filepath.Clean(spec.WorkspacePath),
		},
		AccessMode: spec.AccessMode,
		RuntimeRef: "runtime-" + spec.Name,
	}
	f.environments[spec.Name] = environment
	return environment, nil
}

func (f *fakeEnvironmentService) Delete(_ context.Context, name string) error {
	f.deletes = append(f.deletes, name)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.environments, name)
	return nil
}

type fakeClientService struct {
	environments *fakeEnvironmentService
	state        core.EnvironmentState
	connections  []core.ClientConnection
	statusErr    error
	connectionsErr error
	sshResponse  core.ClientConnection
	sshErr       error
	lastSSH      core.SSHAccessRequest
	forwardResponse core.ClientConnection
	forwardErr   error
	lastForward  core.LocalPortRequest
	unforwardErr error
	unforwarded  []string
}

func (f *fakeClientService) Status(_ context.Context, name string) (core.EnvironmentStatus, error) {
	if f.statusErr != nil {
		return core.EnvironmentStatus{}, f.statusErr
	}
	environment, ok := f.environments.environments[name]
	if !ok {
		return core.EnvironmentStatus{}, core.ErrNotFound
	}
	state := f.state
	if state == "" {
		state = core.EnvironmentRunning
	}
	return core.EnvironmentStatus{Environment: environment, State: state}, nil
}

func (f *fakeClientService) Connections(context.Context, string) ([]core.ClientConnection, error) {
	if f.connectionsErr != nil {
		return nil, f.connectionsErr
	}
	return append([]core.ClientConnection(nil), f.connections...), nil
}

func (f *fakeClientService) Forward(_ context.Context, _ string, req core.LocalPortRequest) (core.ClientConnection, error) {
	f.lastForward = req
	if f.forwardErr != nil {
		return core.ClientConnection{}, f.forwardErr
	}
	return f.forwardResponse, nil
}

func (f *fakeClientService) Unforward(_ context.Context, _ string, id string) error {
	f.unforwarded = append(f.unforwarded, id)
	return f.unforwardErr
}

func (f *fakeClientService) SSH(_ context.Context, _ string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	f.lastSSH = req
	if f.sshErr != nil {
		return core.ClientConnection{}, f.sshErr
	}
	return f.sshResponse, nil
}

type fakeEventReader struct {
	batch  interaction.Batch
	err    error
	offset int64
	limit  int
}

func (f *fakeEventReader) Batch(_ context.Context, offset int64, limit int) (interaction.Batch, error) {
	f.offset = offset
	f.limit = limit
	return f.batch, f.err
}

func TestEnsureCreatesThenReusesExactEnvironment(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{environments: environments}
	adapter := newAdapter(environments, clients, &fakeEventReader{})
	workspace := t.TempDir()

	first, created, err := adapter.Ensure(context.Background(), EnsureRequest{
		Name:          "demo",
		WorkspacePath: workspace,
		AccessMode:    ReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || environments.creates != 1 {
		t.Fatalf("created=%v creates=%d", created, environments.creates)
	}
	if first.Name != "demo" || first.SourceWorkspace != filepath.Clean(workspace) || first.WorkspacePath != WorkspacePath || first.AccessMode != ReadWrite || first.State != Running {
		t.Fatalf("first=%#v", first)
	}

	second, created, err := adapter.Ensure(context.Background(), EnsureRequest{
		Name:          "demo",
		WorkspacePath: workspace,
		AccessMode:    ReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created || environments.creates != 1 || second != first {
		t.Fatalf("second=%#v created=%v creates=%d", second, created, environments.creates)
	}
}

func TestEnsureRefusesAuthorityChangingReuse(t *testing.T) {
	environments := newFakeEnvironmentService()
	workspace := t.TempDir()
	environments.environments["demo"] = core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{ID: "workspace:demo", Path: workspace},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "runtime-demo",
	}
	clients := &fakeClientService{environments: environments}
	adapter := newAdapter(environments, clients, &fakeEventReader{})

	for name, req := range map[string]EnsureRequest{
		"access-mode": {Name: "demo", WorkspacePath: workspace, AccessMode: ReadOnly},
		"workspace":   {Name: "demo", WorkspacePath: t.TempDir(), AccessMode: ReadWrite},
	} {
		t.Run(name, func(t *testing.T) {
			_, created, err := adapter.Ensure(context.Background(), req)
			if created || !errors.Is(err, ErrAlreadyExists) {
				t.Fatalf("created=%v err=%v", created, err)
			}
		})
	}
}

func TestPrepareSSHAcceptsOnlyPublicMaterialAndReturnsLoopback(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{
		environments: environments,
		sshResponse: core.ClientConnection{
			ID: "ssh-2222", Kind: "ssh", Host: "127.0.0.1", Port: 2222, TargetPort: 22, User: "root",
		},
	}
	adapter := newAdapter(environments, clients, &fakeEventReader{})
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestOnly adapter@test"

	connection, err := adapter.PrepareSSH(context.Background(), SSHRequest{
		Environment: "demo",
		PublicKey:   publicKey,
		HostPort:    2222,
	})
	if err != nil {
		t.Fatal(err)
	}
	if clients.lastSSH.PublicKey != publicKey || clients.lastSSH.HostPort != 2222 {
		t.Fatalf("request=%#v", clients.lastSSH)
	}
	if connection.Host != "127.0.0.1" || connection.Kind != "ssh" || connection.TargetPort != 22 || connection.User != "root" {
		t.Fatalf("connection=%#v", connection)
	}
}

func TestPrepareSSHRejectsAndRevokesNonLoopbackProviderResult(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{
		environments: environments,
		sshResponse: core.ClientConnection{
			ID: "ssh-2222", Kind: "ssh", Host: "0.0.0.0", Port: 2222, TargetPort: 22, User: "root",
		},
	}
	adapter := newAdapter(environments, clients, &fakeEventReader{})

	_, err := adapter.PrepareSSH(context.Background(), SSHRequest{Environment: "demo", PublicKey: "ssh-ed25519 AAAA test", HostPort: 2222})
	if !errors.Is(err, ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
	if len(clients.unforwarded) != 1 || clients.unforwarded[0] != "ssh-2222" {
		t.Fatalf("unforwarded=%#v", clients.unforwarded)
	}
}

func TestPrepareSSHCleanupFailureRequiresRecovery(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{
		environments: environments,
		sshResponse: core.ClientConnection{
			ID: "ssh-2222", Kind: "ssh", Host: "0.0.0.0", Port: 2222, TargetPort: 22,
		},
		unforwardErr: errors.New("cannot revoke"),
	}
	adapter := newAdapter(environments, clients, &fakeEventReader{})

	_, err := adapter.PrepareSSH(context.Background(), SSHRequest{Environment: "demo", PublicKey: "ssh-ed25519 AAAA test", HostPort: 2222})
	if !errors.Is(err, ErrIncompatibleState) || !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("err=%v want incompatible+recovery", err)
	}
}

func TestConnectionsRejectNonLoopbackDrift(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{
		environments: environments,
		connections: []core.ClientConnection{{ID: "web", Kind: "tcp", Host: "192.0.2.10", Port: 8080, TargetPort: 8080}},
	}
	adapter := newAdapter(environments, clients, &fakeEventReader{})

	_, err := adapter.Connections(context.Background(), "demo")
	if !errors.Is(err, ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestForwardValidatesExpectedLoopbackMetadata(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{
		environments: environments,
		forwardResponse: core.ClientConnection{ID: "web", Kind: "tcp", Host: "127.0.0.1", Port: 8080, TargetPort: 3000},
	}
	adapter := newAdapter(environments, clients, &fakeEventReader{})

	connection, err := adapter.Forward(context.Background(), ForwardRequest{Environment: "demo", HostPort: 8080, TargetPort: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if clients.lastForward.Protocol != "tcp" || connection.Host != "127.0.0.1" || connection.TargetPort != 3000 {
		t.Fatalf("request=%#v connection=%#v", clients.lastForward, connection)
	}
}

func TestInteractionBatchUsesPublicInteractionContract(t *testing.T) {
	environments := newFakeEnvironmentService()
	clients := &fakeClientService{environments: environments}
	events := &fakeEventReader{batch: interaction.Batch{SchemaVersion: interaction.SchemaVersion, NextOffset: 42}}
	adapter := newAdapter(environments, clients, events)

	batch, err := adapter.InteractionBatch(context.Background(), 12, 0)
	if err != nil {
		t.Fatal(err)
	}
	if events.offset != 12 || events.limit != interaction.DefaultBatchSize || batch.NextOffset != 42 {
		t.Fatalf("offset=%d limit=%d batch=%#v", events.offset, events.limit, batch)
	}
}

func TestTranslateErrorUsesPublicSentinels(t *testing.T) {
	for name, tc := range map[string]struct {
		source error
		want   error
	}{
		"not-found": {core.ErrNotFound, ErrNotFound},
		"invalid":   {core.ErrInvalidArgument, ErrInvalidArgument},
		"busy":      {core.ErrWorkspaceBusy, ErrBusy},
		"recovery":  {core.ErrRecoveryRequired, ErrRecoveryRequired},
	} {
		t.Run(name, func(t *testing.T) {
			if err := translateError(tc.source); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
		})
	}
}
