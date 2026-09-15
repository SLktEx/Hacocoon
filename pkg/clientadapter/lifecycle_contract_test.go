package clientadapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type verificationClients struct {
	*fakeClientService
	after core.EnvironmentStatus
	err   error
}

func (c *verificationClients) Status(ctx context.Context, name string) (core.EnvironmentStatus, error) {
	if c.environments.creates == 0 {
		return c.fakeClientService.Status(ctx, name)
	}
	return c.after, c.err
}

type cleanupEnvironments struct {
	*fakeEnvironmentService
	cancel            context.CancelFunc
	cleanupContextErr error
}

func (s *cleanupEnvironments) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	environment, err := s.fakeEnvironmentService.Create(ctx, spec)
	if s.cancel != nil {
		s.cancel()
	}
	return environment, err
}

func (s *cleanupEnvironments) Delete(ctx context.Context, name string) error {
	s.cleanupContextErr = ctx.Err()
	return s.fakeEnvironmentService.Delete(ctx, name)
}

func TestEnsureFailedVerificationCleansOnlyItsCreationDespiteCancellation(t *testing.T) {
	workspace := t.TempDir()
	valid := core.EnvironmentStatus{Environment: core.Environment{Name: "new", Workspace: core.Workspace{Path: workspace}, AccessMode: core.WorkspaceReadOnly}, State: core.EnvironmentRunning}
	for _, failure := range []string{"unavailable", "access", "state", "name", "workspace"} {
		for _, cleanupFailure := range []bool{false, true} {
			t.Run(failure+"/cleanup-failure="+strconv.FormatBool(cleanupFailure), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				environments := &cleanupEnvironments{fakeEnvironmentService: newFakeEnvironmentService(), cancel: cancel}
				environments.environments["existing"] = core.Environment{Name: "existing", RuntimeRef: "keep-this-runtime"}
				if cleanupFailure {
					environments.deleteErr = errors.New("owned runtime absence not confirmed")
				}
				clients := &verificationClients{fakeClientService: &fakeClientService{environments: environments.fakeEnvironmentService}, after: valid}
				want := ErrIncompatibleState
				switch failure {
				case "unavailable":
					clients.err, want = core.ErrRuntimeUnavailable, ErrUnavailable
				case "access":
					clients.after.Environment.AccessMode = "unexpected"
				case "state":
					clients.after.State = "unexpected"
				case "name":
					clients.after.Environment.Name = ""
				case "workspace":
					clients.after.Environment.Workspace.Path = ""
				}
				adapter := newAdapter(environments, clients, nil)
				result, created, err := adapter.Ensure(ctx, EnsureRequest{Name: "new", WorkspacePath: workspace, AccessMode: ReadOnly})
				if created || result != (Environment{}) || !errors.Is(err, want) || errors.Is(err, ErrRecoveryRequired) != cleanupFailure {
					t.Fatalf("failed verification reported success or lost recovery: result=%+v created=%v error=%v", result, created, err)
				}
				_, retained := environments.environments["new"]
				if environments.cleanupContextErr != nil || retained != cleanupFailure || environments.environments["existing"].RuntimeRef != "keep-this-runtime" {
					t.Fatal("cleanup lost ownership or used cancelled context", environments.environments, environments.cleanupContextErr)
				}
			})
		}
	}
}

func TestEnsureRefusesUnverifiableExistingStateWithoutCreatingOrDeleting(t *testing.T) {
	workspace := t.TempDir()
	for _, failure := range []string{"status", "create", "state", "mode"} {
		environments := newFakeEnvironmentService()
		clients := &fakeClientService{environments: environments}
		want := ErrUnavailable
		switch failure {
		case "status":
			clients.statusErr = core.ErrRuntimeUnavailable
		case "create":
			environments.createErr = core.ErrRuntimeUnavailable
		default:
			want = ErrIncompatibleState
			environments.environments["demo"] = core.Environment{Name: "demo", Workspace: core.Workspace{Path: workspace}, AccessMode: core.WorkspaceReadWrite}
			if failure == "state" {
				clients.state = "unexpected"
			} else {
				environment := environments.environments["demo"]
				environment.AccessMode = "unexpected"
				environments.environments["demo"] = environment
			}
		}
		_, created, err := newAdapter(environments, clients, nil).Ensure(context.Background(), EnsureRequest{Name: "demo", WorkspacePath: workspace})
		if created || !errors.Is(err, want) || environments.creates != 0 || len(environments.deletes) != 0 {
			t.Fatal(failure, created, err, environments)
		}
	}
}

func TestAdapterStatusDeleteAndReadOnlyEventsUsePublicResults(t *testing.T) {
	ctx := context.Background()
	environments := newFakeEnvironmentService()
	workspace := t.TempDir()
	environments.environments["demo"] = core.Environment{Name: "demo", Workspace: core.Workspace{Path: workspace}, AccessMode: core.WorkspaceReadOnly}
	clients := &fakeClientService{environments: environments, state: core.EnvironmentStopped}
	eventFailure := errors.New("event reader unavailable")
	adapter := newAdapter(environments, clients, &fakeEventReader{err: eventFailure})
	status, err := adapter.Status(ctx, "demo")
	if err != nil || status != (Environment{Name: "demo", SourceWorkspace: workspace, WorkspacePath: WorkspacePath, AccessMode: ReadOnly, State: Stopped}) {
		t.Fatal(status, err)
	}
	if _, err := adapter.Status(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := adapter.InteractionBatch(ctx, 12, 7); !errors.Is(err, eventFailure) {
		t.Fatal(err)
	}
	environments.deleteErr = core.ErrRecoveryRequired
	if err := adapter.Delete(ctx, "demo"); !errors.Is(err, ErrRecoveryRequired) || len(environments.environments) != 1 {
		t.Fatal(err)
	}
	environments.deleteErr = nil
	if err := adapter.Delete(ctx, "demo"); err != nil || len(environments.environments) != 0 {
		t.Fatal(err)
	}
}

func TestPublicAdapterRejectsMissingServicesAndInvalidRequests(t *testing.T) {
	ctx := context.Background()
	operations := []func(*Adapter) error{
		func(a *Adapter) error { _, _, err := a.Ensure(ctx, EnsureRequest{}); return err },
		func(a *Adapter) error { _, err := a.Status(ctx, " "); return err },
		func(a *Adapter) error { _, err := a.Connections(ctx, " "); return err },
		func(a *Adapter) error { _, err := a.PrepareSSH(ctx, SSHRequest{}); return err },
		func(a *Adapter) error { _, err := a.Forward(ctx, ForwardRequest{}); return err },
		func(a *Adapter) error { return a.Revoke(ctx, "", "") },
		func(a *Adapter) error { return a.Delete(ctx, "") },
		func(a *Adapter) error { _, err := a.InteractionBatch(ctx, -1, -1); return err },
	}
	environments := newFakeEnvironmentService()
	for _, adapter := range []*Adapter{nil, {}, newControllerAdapter(nil), newAdapter(environments, &fakeClientService{environments: environments}, &fakeEventReader{})} {
		for i, operation := range operations {
			if err := operation(adapter); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("operation %d: %v", i, err)
			}
		}
	}
	adapter := newAdapter(environments, &fakeClientService{environments: environments}, nil)
	for _, request := range []ForwardRequest{{Environment: "demo", TargetPort: 0}, {Environment: "demo", TargetPort: 65536}, {Environment: "demo", TargetPort: 22, HostPort: -1}, {Environment: "demo", TargetPort: 22, HostPort: 65536}} {
		if _, err := adapter.Forward(ctx, request); !errors.Is(err, ErrInvalidArgument) {
			t.Fatal(request, err)
		}
	}
	if _, _, err := adapter.Ensure(ctx, EnsureRequest{Name: "demo", WorkspacePath: t.TempDir(), AccessMode: "owner"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", file, filepath.Join(t.TempDir(), "missing")} {
		if _, _, err := adapter.Ensure(ctx, EnsureRequest{Name: "demo", WorkspacePath: path}); err == nil {
			t.Fatal("accepted invalid Workspace", path)
		}
	}
	if environments.creates != 0 {
		t.Fatal("invalid request created an Environment")
	}
}

func TestStreamTargetTokenPreservesBindingAndRejectsIncompleteIdentity(t *testing.T) {
	source := StreamTarget{Environment: "demo", Instance: "env-" + strings.Repeat("a", 32), Workspace: "workspace:demo", AccessMode: "rw", Service: "ssh", Grant: "ssh-one"}
	token, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := core.DecodeStreamTarget(token)
	if err != nil || decoded.Environment != source.Environment || decoded.Instance != source.Instance || string(decoded.Workspace) != source.Workspace || string(decoded.AccessMode) != source.AccessMode || decoded.Service != source.Service || decoded.Grant != source.Grant {
		t.Fatal("token lost its exact binding", decoded, err)
	}
	source.Instance = ""
	if _, err := source.Token(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
}

func TestConnectionPreparationRejectsMismatchedRepliesAndRetainsRecovery(t *testing.T) {
	for _, mode := range []string{"forward", "ssh"} {
		for _, failure := range []string{"wrong-port", "wrong-kind", "invalid-address", "missing-id"} {
			for _, revokeFailure := range []bool{false, true} {
				t.Run(mode+"/"+failure+"/revoke-failure="+strconv.FormatBool(revokeFailure), func(t *testing.T) {
					raw := core.ClientConnection{ID: "new-grant", Kind: "tcp", Host: "127.0.0.1", Port: 3456, TargetPort: 3000}
					if mode == "ssh" {
						raw = core.ClientConnection{ID: "new-grant", Kind: "ssh", Target: testStreamTarget(), TargetPort: 22, User: "root"}
					}
					switch failure {
					case "wrong-port":
						raw.TargetPort++
					case "wrong-kind":
						if mode == "forward" {
							raw = core.ClientConnection{ID: "new-grant", Kind: "ssh", Target: testStreamTarget(), TargetPort: 22}
						} else {
							raw = core.ClientConnection{ID: "new-grant", Kind: "tcp", Host: "127.0.0.1", Port: 3456, TargetPort: 22}
						}
					case "invalid-address":
						raw.Host = "0.0.0.0"
					case "missing-id":
						raw.ID = ""
					}
					clients := &fakeClientService{environments: newFakeEnvironmentService(), forwardResponse: raw, sshResponse: raw}
					if revokeFailure {
						clients.unforwardErr = errors.New("connection absence unconfirmed")
					}
					adapter := newAdapter(clients.environments, clients, nil)
					var result Connection
					var err error
					if mode == "forward" {
						result, err = adapter.Forward(context.Background(), ForwardRequest{Environment: "demo", HostPort: 3456, TargetPort: 3000})
					} else {
						result, err = adapter.PrepareSSH(context.Background(), SSHRequest{Environment: "demo", PublicKey: "ssh-ed25519 AAAA test"})
					}
					if result != (Connection{}) || !errors.Is(err, ErrIncompatibleState) || errors.Is(err, ErrRecoveryRequired) != (revokeFailure || failure == "missing-id") {
						t.Fatal("incompatible connection escaped or recovery was lost", result, err)
					}
					if raw.ID == "" {
						if len(clients.unforwarded) != 0 {
							t.Fatal("guessed a missing grant identity", clients.unforwarded)
						}
					} else if len(clients.unforwarded) != 1 || clients.unforwarded[0] != raw.ID {
						t.Fatal("revoked a different connection", clients.unforwarded)
					}
				})
			}
		}
	}
}

func TestConnectionReadAndPreparationFailuresReturnPublicErrors(t *testing.T) {
	clients := &fakeClientService{environments: newFakeEnvironmentService(), connectionsErr: core.ErrRuntimeUnavailable, forwardErr: core.ErrWorkspaceBusy, sshErr: core.ErrStorageBusy}
	adapter := newAdapter(clients.environments, clients, nil)
	if _, err := adapter.Connections(context.Background(), "demo"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := adapter.Forward(context.Background(), ForwardRequest{Environment: "demo", HostPort: 3456, TargetPort: 3000}); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if _, err := adapter.PrepareSSH(context.Background(), SSHRequest{Environment: "demo", PublicKey: "ssh-ed25519 AAAA test"}); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if len(clients.unforwarded) != 0 {
		t.Fatal("invented a connection after backend failure", clients.unforwarded)
	}
}
