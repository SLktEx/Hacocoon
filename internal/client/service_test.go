package client

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const validEd25519Key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"

type fakeStore struct {
	environment core.Environment
	err         error
}

func (f fakeStore) GetEnvironment(context.Context, string) (core.Environment, error) {
	return f.environment, f.err
}

type fakeRuntime struct {
	status      core.EnvironmentRuntimeStatus
	statusRef   string
	listRef     string
	connections []core.ClientConnection
	forwardRef  string
	forwardReq  core.LocalPortRequest
	removeRef   string
	removeID    string
	sshRef      string
	sshReq      core.SSHAccessRequest
	revokeRef   string
	revokeID    string
	connection  core.ClientConnection
}

func (f *fakeRuntime) InspectEnvironment(_ context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	f.statusRef = ref
	return f.status, nil
}
func (f *fakeRuntime) ListClientConnections(_ context.Context, ref string) ([]core.ClientConnection, error) {
	f.listRef = ref
	return append([]core.ClientConnection(nil), f.connections...), nil
}
func (f *fakeRuntime) ForwardLocalPort(_ context.Context, ref string, req core.LocalPortRequest) (core.ClientConnection, error) {
	f.forwardRef, f.forwardReq = ref, req
	return f.connection, nil
}
func (f *fakeRuntime) RemoveClientConnection(_ context.Context, ref, id string) error {
	f.removeRef, f.removeID = ref, id
	return nil
}
func (f *fakeRuntime) PrepareSSHAccess(_ context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	f.sshRef, f.sshReq = ref, req
	return f.connection, nil
}
func (f *fakeRuntime) RevokeSSHAccess(_ context.Context, ref, id string) error {
	f.revokeRef, f.revokeID = ref, id
	return nil
}

func TestStatusCombinesEnvironmentAndObservedState(t *testing.T) {
	runtime := &fakeRuntime{status: core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	status, err := service.Status(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if status.Environment.Name != "demo" || status.State != core.EnvironmentRunning || runtime.statusRef != "haco-demo" {
		t.Fatalf("status=%#v runtime ref=%q", status, runtime.statusRef)
	}
}

func TestConnectionsReconcilesRuntimeDevices(t *testing.T) {
	runtime := &fakeRuntime{connections: []core.ClientConnection{{ID: "ssh-2222", Kind: "ssh"}}}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	connections, err := service.Connections(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.listRef != "haco-demo" || len(connections) != 1 || connections[0].ID != "ssh-2222" {
		t.Fatalf("ref=%q connections=%#v", runtime.listRef, connections)
	}
}

func TestForwardValidatesPortsAndUsesStoredRuntimeRef(t *testing.T) {
	runtime := &fakeRuntime{connection: core.ClientConnection{ID: "tcp-8080-3000"}}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	connection, err := service.Forward(context.Background(), "demo", core.LocalPortRequest{Protocol: "tcp", HostPort: 8080, TargetPort: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if connection.ID != "tcp-8080-3000" || runtime.forwardRef != "haco-demo" || runtime.forwardReq.TargetPort != 3000 {
		t.Fatalf("connection=%#v ref=%q req=%#v", connection, runtime.forwardRef, runtime.forwardReq)
	}
	_, err = service.Forward(context.Background(), "demo", core.LocalPortRequest{Protocol: "udp", HostPort: 1, TargetPort: 1})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("udp error=%v", err)
	}
}

func TestForwardNormalizesDefaultProtocolBeforeRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	if _, err := service.Forward(context.Background(), "demo", core.LocalPortRequest{HostPort: 8080, TargetPort: 3000}); err != nil {
		t.Fatal(err)
	}
	if runtime.forwardReq.Protocol != "tcp" {
		t.Fatalf("runtime protocol = %q", runtime.forwardReq.Protocol)
	}
}

func TestSSHCanonicalizesValidPublicKeyAndRejectsMalformedPayload(t *testing.T) {
	runtime := &fakeRuntime{connection: core.ClientConnection{Kind: "ssh"}}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	withComment := validEd25519Key + " user@example"
	if _, err := service.SSH(context.Background(), "demo", core.SSHAccessRequest{PublicKey: withComment + "\n", HostPort: 2222}); err != nil {
		t.Fatal(err)
	}
	if runtime.sshRef != "haco-demo" || runtime.sshReq.PublicKey != validEd25519Key {
		t.Fatalf("ssh ref=%q req=%#v", runtime.sshRef, runtime.sshReq)
	}
	for _, invalid := range []string{"", "not-a-key", "ssh-ed25519 not-base64", withComment + "\n" + withComment} {
		if _, err := service.SSH(context.Background(), "demo", core.SSHAccessRequest{PublicKey: invalid, HostPort: 2222}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("invalid key %q error=%v", invalid, err)
		}
	}
}

func TestUnforwardRevokesSSHCredentialAndTransport(t *testing.T) {
	runtime := &fakeRuntime{}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	if err := service.Unforward(context.Background(), "demo", "ssh-2222"); err != nil {
		t.Fatal(err)
	}
	if runtime.revokeRef != "haco-demo" || runtime.revokeID != "ssh-2222" || runtime.removeID != "" {
		t.Fatalf("revoke ref=%q id=%q remove=%q", runtime.revokeRef, runtime.revokeID, runtime.removeID)
	}
}

func TestUnforwardRemovesOrdinaryProxy(t *testing.T) {
	runtime := &fakeRuntime{}
	service := New(runtime, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	if err := service.Unforward(context.Background(), "demo", "tcp-8080-3000"); err != nil {
		t.Fatal(err)
	}
	if runtime.removeRef != "haco-demo" || runtime.removeID != "tcp-8080-3000" || runtime.revokeID != "" {
		t.Fatalf("remove ref=%q id=%q revoke=%q", runtime.removeRef, runtime.removeID, runtime.revokeID)
	}
}

func TestUnforwardRejectsUnsafeConnectionID(t *testing.T) {
	service := New(&fakeRuntime{}, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}})
	if err := service.Unforward(context.Background(), "demo", "../oops"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}
