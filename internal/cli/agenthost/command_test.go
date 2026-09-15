package agenthostcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/agenthost"
	"github.com/SLktEx/Hacocoon/internal/client"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Keep the real session broker, durable bindings and client service. Only the
// Environment and provider boundary is substituted for command-level contracts.
type commandBackend struct {
	environments                                                 map[string]core.Environment
	connections                                                  []core.ClientConnection
	created, prepared, deleted                                   int
	revoked                                                      []string
	state                                                        core.EnvironmentState
	createErr, inspectErr, listErr, sshErr, revokeErr, deleteErr error
	publicKey                                                    string
	nextConnection                                               *core.ClientConnection
}

func (b *commandBackend) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if b.createErr != nil {
		return core.Environment{}, b.createErr
	}
	b.created++
	e := core.Environment{Name: spec.Name, RuntimeRef: spec.Name, AccessMode: spec.AccessMode, Workspace: core.Workspace{ID: "work", Path: spec.WorkspacePath}, CreatedAt: time.Unix(1, 0).UTC()}
	b.environments[e.Name] = e
	return e, nil
}
func (b *commandBackend) Delete(_ context.Context, name string) error {
	if b.deleteErr != nil {
		return b.deleteErr
	}
	b.deleted++
	delete(b.environments, name)
	return nil
}
func (b *commandBackend) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	e, ok := b.environments[name]
	if !ok {
		return e, core.ErrNotFound
	}
	return e, nil
}
func (b *commandBackend) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	return core.EnvironmentRuntimeStatus{State: b.state}, b.inspectErr
}
func (b *commandBackend) ListClientConnections(context.Context, string) ([]core.ClientConnection, error) {
	return b.connections, b.listErr
}
func (b *commandBackend) ForwardLocalPort(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error) {
	return core.ClientConnection{}, core.ErrUnsupported
}
func (b *commandBackend) RemoveClientConnection(context.Context, string, string) error {
	return core.ErrUnsupported
}
func (b *commandBackend) PrepareSSHAccess(_ context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	if b.sshErr != nil {
		return core.ClientConnection{}, b.sshErr
	}
	b.prepared++
	b.publicKey = req.PublicKey
	c := testAgentConnection()
	c.ID = fmt.Sprintf("ssh-%d", b.prepared)
	c.Target.Environment, c.Target.Grant, c.Target.AccessMode = ref, c.ID, b.environments[ref].AccessMode
	if b.nextConnection != nil {
		c = *b.nextConnection
	}
	b.connections = append(b.connections, c)
	return c, nil
}
func (b *commandBackend) RevokeSSHAccess(_ context.Context, _ string, id string) error {
	b.revoked = append(b.revoked, id)
	return b.revokeErr
}

type commandFixture struct {
	app                                *composition.App
	backend                            *commandBackend
	home, workspace, identity, session string
}

func newCommandFixture(t *testing.T) commandFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("WSL_DISTRO_NAME", "")
	identity := filepath.Join(home, ".ssh", "id_ed25519")
	if err := os.MkdirAll(filepath.Dir(identity), 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{identity: "client-only private key", identity + ".pub": testAgentConnection().HostPublicKey} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b := &commandBackend{environments: map[string]core.Environment{}, state: core.EnvironmentRunning}
	app := &composition.App{AgentHosts: agenthost.New(b, b, agenthost.NewJSONBindingStore(filepath.Join(home, "bindings.json"))), Clients: client.New(b, b)}
	return commandFixture{app: app, backend: b, home: home, workspace: t.TempDir(), identity: identity, session: "opaque-private-session"}
}

func captureCommand(t *testing.T, run func() error) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = original; _ = f.Close() }()
	runErr := run()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), runErr
}

func (f commandFixture) prepare(t *testing.T, flags ...string) (string, error) {
	args := append([]string{"prepare", "--session", f.session, "--no-launch"}, flags...)
	args = append(args, f.workspace)
	return captureCommand(t, func() error { return dispatch(context.Background(), f.app, args) })
}

func TestAgentCommandsPrepareLookupReuseAndRelease(t *testing.T) {
	f := newCommandFixture(t)
	raw, err := f.prepare(t, "--json", "--read-only")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor agentSessionDescriptor
	if err := json.Unmarshal([]byte(raw), &descriptor); err != nil {
		t.Fatal("JSON output contains non-JSON diagnostics", raw, err)
	}
	if descriptor.SessionID != f.session || descriptor.WorkspacePath != f.workspace || descriptor.RemoteWorkspace != "/workspace" || descriptor.SSHAlias != agentSSHAlias(f.session) || !strings.Contains(descriptor.FolderURI, descriptor.SSHAlias) {
		t.Fatal("session descriptor lost binding", descriptor)
	}
	if f.backend.created != 1 || f.backend.prepared != 1 || f.backend.publicKey != testAgentConnection().HostPublicKey || f.backend.environments[descriptor.Environment].AccessMode != core.WorkspaceReadOnly {
		t.Fatal("wrong creation mode or public-key boundary", f.backend)
	}
	lookup, err := captureCommand(t, func() error {
		return dispatch(context.Background(), f.app, []string{"lookup", "--session", f.session, "--json"})
	})
	if err != nil || lookup != raw || f.backend.created != 1 {
		t.Fatal("lookup mutated or rebound session", lookup, err)
	}
	text, err := f.prepare(t, "--read-only")
	if err != nil || strings.Contains(text, f.session) || strings.Contains(text, "New -> Remote") || !strings.Contains(text, "folder-uri: "+descriptor.FolderURI) || f.backend.prepared != 1 {
		t.Fatal("prepare did not reuse exact grant or render current descriptor", text, err)
	}
	path := managedConfigPath(f.home, descriptor.SSHAlias)
	if _, err := readManagedSSHConfig(path); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		out, err := captureCommand(t, func() error {
			return dispatch(context.Background(), f.app, []string{"release", "--session", f.session})
		})
		if err != nil || out != "released: "+descriptor.SSHAlias+"\n" || f.backend.deleted != 1 {
			t.Fatal("release is not idempotent", out, err)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("managed fragment survived release", err)
	}
	if _, err := f.app.AgentHosts.Lookup(context.Background(), f.session); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("released session remains bound", err)
	}
	if content, err := os.ReadFile(f.identity); err != nil || string(content) != "client-only private key" {
		t.Fatal("release modified client identity", err)
	}
}

func TestAgentCommandsRejectInvalidArgumentsBeforeAcquiring(t *testing.T) {
	f := newCommandFixture(t)
	for _, args := range [][]string{nil, {"unknown"}, {"prepare"}, {"prepare", "--unknown"}, {"prepare", "--session", "s", "a", "b"}, {"prepare", "--session", "s", "--json=invalid"}, {"prepare", "--session", "s", "--no-launch=invalid"}, {"prepare", "--session", "s", "--code", " "}, {"lookup"}, {"lookup", "--unknown"}, {"lookup", "--session", "s", "extra"}, {"release"}, {"release", "--unknown"}, {"release", "--session", "s", "extra"}} {
		if err := dispatch(context.Background(), f.app, args); err == nil || f.backend.created != 0 || f.backend.deleted != 0 {
			t.Fatal("invalid arguments caused side effects", args, err)
		}
	}
	for _, name := range []string{"prepare", "lookup", "release"} {
		if err := dispatch(context.Background(), f.app, []string{name, "--session", "missing", "--identity"}); err == nil {
			t.Fatal("missing/unknown flag accepted", name)
		}
	}
}

func TestAgentSSHRotationRetainsEnvironmentAndRevokesOnlyOldGrant(t *testing.T) {
	f := newCommandFixture(t)
	if _, err := f.prepare(t); err != nil {
		t.Fatal(err)
	}
	old, err := readManagedSSHConfig(managedConfigPath(f.home, agentSSHAlias(f.session)))
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(f.home, ".ssh", "rotated")
	for path, data := range map[string]string{other: "new client-only private key", other + ".pub": testAgentConnection().HostPublicKey} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.prepare(t, "--identity", other); err != nil {
		t.Fatal(err)
	}
	current, err := readManagedSSHConfig(managedConfigPath(f.home, agentSSHAlias(f.session)))
	if err != nil || current.Connection.ID == old.Connection.ID || current.IdentityFile != "~/.ssh/rotated" || f.backend.created != 1 || len(f.backend.revoked) != 1 || f.backend.revoked[0] != old.Connection.ID {
		t.Fatal("rotation lost exact grant or environment", current, f.backend.revoked, err)
	}
}

func TestAgentSSHRotationRejectsChangedOwnershipBeforeReplacingConfig(t *testing.T) {
	for _, field := range []string{"environment", "instance", "workspace", "access", "service", "missing"} {
		t.Run(field, func(t *testing.T) {
			f := newCommandFixture(t)
			if _, err := f.prepare(t); err != nil {
				t.Fatal(err)
			}
			path := managedConfigPath(f.home, agentSSHAlias(f.session))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			c := f.backend.connections[0]
			target := *c.Target
			c.ID, target.Grant = "ssh-replacement", "ssh-replacement"
			c.Target = &target
			switch field {
			case "environment":
				target.Environment = "other"
			case "instance":
				target.Instance = "env-" + strings.Repeat("b", 32)
			case "workspace":
				target.Workspace = "other"
			case "access":
				target.AccessMode = core.WorkspaceReadOnly
			case "service":
				target.Service = "other"
			case "missing":
				c.Target = nil
			}
			f.backend.nextConnection = &c
			// A disappeared grant forces preparation instead of exact-grant reuse.
			f.backend.connections = nil
			if _, err := f.prepare(t); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("changed ownership accepted", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) || len(f.backend.revoked) != 1 || f.backend.revoked[0] != c.ID || f.backend.deleted != 0 {
				t.Fatal("refusal changed previous ownership", string(after), f.backend.revoked, err)
			}
		})
	}
}

func TestAgentPrepareRefusalsDoNotPublishConnection(t *testing.T) {
	for _, stage := range []string{"workspace-missing", "workspace-file", "identity-missing", "identity-directory", "public-key-missing", "create", "inspect", "stopped", "connections", "ssh", "include", "pin", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			f := newCommandFixture(t)
			failure := core.ErrRuntimeUnavailable
			wantPrepared, wantRevoked := 0, 0
			switch stage {
			case "workspace-missing":
				f.workspace = filepath.Join(f.home, "missing")
			case "workspace-file":
				f.workspace = f.identity
			case "identity-missing":
				if err := os.Remove(f.identity); err != nil {
					t.Fatal(err)
				}
			case "identity-directory":
				if err := os.Remove(f.identity); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(f.identity, 0700); err != nil {
					t.Fatal(err)
				}
			case "public-key-missing":
				if err := os.Remove(f.identity + ".pub"); err != nil {
					t.Fatal(err)
				}
			case "create":
				f.backend.createErr = failure
			case "inspect":
				f.backend.inspectErr = failure
			case "stopped":
				f.backend.state = core.EnvironmentStopped
			case "connections":
				f.backend.listErr = failure
			case "ssh":
				f.backend.sshErr = failure
			case "include", "cleanup":
				if err := os.Mkdir(filepath.Join(f.home, ".ssh", "config"), 0700); err != nil {
					t.Fatal(err)
				}
				wantPrepared, wantRevoked = 1, 1
				if stage == "cleanup" {
					f.backend.revokeErr = failure
				}
			case "pin":
				dir := filepath.Join(f.home, ".ssh", "hacocoon")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, agentSSHAlias(f.session)+".known_hosts"), []byte("previous pinned key\n"), 0600); err != nil {
					t.Fatal(err)
				}
				wantPrepared, wantRevoked = 1, 1
			}
			out, err := f.prepare(t)
			if err == nil || out != "" || f.backend.prepared != wantPrepared || len(f.backend.revoked) != wantRevoked {
				t.Fatal("failed preparation published or revoked wrong grant", out, err, f.backend)
			}
			if stage == "cleanup" && (!errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, failure)) {
				t.Fatal("ambiguous cleanup was hidden", err)
			}
			if _, err := os.Stat(managedConfigPath(f.home, agentSSHAlias(f.session))); !os.IsNotExist(err) {
				t.Fatal("failure published managed config", err)
			}
		})
	}
}

func TestAgentPrepareKeepsReusedGrantWhenClientConfigFails(t *testing.T) {
	f := newCommandFixture(t)
	if _, err := f.prepare(t); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(f.home, ".ssh", "config")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := f.prepare(t); err == nil || f.backend.prepared != 1 || len(f.backend.revoked) != 0 {
		t.Fatal("existing connection revoked on client failure", err, f.backend)
	}
}

func TestAgentReleaseRetainsClientConfigWhenEnvironmentDeletionFails(t *testing.T) {
	f := newCommandFixture(t)
	if _, err := f.prepare(t); err != nil {
		t.Fatal(err)
	}
	f.backend.deleteErr = core.ErrRecoveryRequired
	if _, err := captureCommand(t, func() error {
		return dispatch(context.Background(), f.app, []string{"release", "--session", f.session})
	}); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	if _, err := readManagedSSHConfig(managedConfigPath(f.home, agentSSHAlias(f.session))); err != nil {
		t.Fatal("failed release lost client config", err)
	}
	if _, err := f.app.AgentHosts.Lookup(context.Background(), f.session); err != nil {
		t.Fatal("failed release lost binding", err)
	}
}

func TestAgentCommandsRefuseForeignOrMalformedManagedFragments(t *testing.T) {
	for _, mode := range []string{"alias", "distribution", "unmarked", "invalid-json", "directory"} {
		t.Run(mode, func(t *testing.T) {
			f := newCommandFixture(t)
			if _, err := f.prepare(t); err != nil {
				t.Fatal(err)
			}
			path := managedConfigPath(f.home, agentSSHAlias(f.session))
			managed, err := readManagedSSHConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "alias":
				managed.Alias = "unrelated-client"
			case "distribution":
				managed.Distro = "unrelated-distribution"
			}
			meta, err := json.Marshal(managed)
			if err != nil {
				t.Fatal(err)
			}
			content := "# Hacocoon agent connection " + string(meta) + "\n"
			if mode == "unmarked" {
				content = "Host personal\n"
			}
			if mode == "invalid-json" {
				content = "# Hacocoon agent connection {\n"
			}
			if mode == "directory" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.prepare(t); err == nil || f.backend.prepared != 1 || len(f.backend.revoked) != 0 {
				t.Fatal("foreign config adopted", err)
			}
			if err := dispatch(context.Background(), f.app, []string{"release", "--session", f.session}); err == nil || f.backend.deleted != 1 {
				t.Fatal("unresolved fragment cleanup reported success", err)
			}
			if mode != "directory" {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != content {
					t.Fatal("unowned config removed", string(got), err)
				}
			}
		})
	}
}

func TestAgentRotationReportsRecoveryWithReplacementReadyWhenOldRevokeFails(t *testing.T) {
	f := newCommandFixture(t)
	if _, err := f.prepare(t); err != nil {
		t.Fatal(err)
	}
	f.backend.revokeErr = core.ErrRuntimeUnavailable
	// Force a fresh grant while retaining the old grant in the provider listing.
	other := filepath.Join(f.home, ".ssh", "other")
	for path, data := range map[string]string{other: "client-only", other + ".pub": testAgentConnection().HostPublicKey} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.prepare(t, "--identity", other); !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, f.backend.revokeErr) {
		t.Fatal("failed old-grant revoke hidden", err)
	}
	managed, err := readManagedSSHConfig(managedConfigPath(f.home, agentSSHAlias(f.session)))
	if err != nil || managed.Connection.ID != "ssh-2" || len(f.backend.revoked) != 1 || f.backend.revoked[0] != "ssh-1" {
		t.Fatal("replacement was lost or revoked", managed, err, f.backend.revoked)
	}
}
