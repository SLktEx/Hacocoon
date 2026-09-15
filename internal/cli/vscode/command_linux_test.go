//go:build linux

package vscodecli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/pkg/clientadapter"
)

const commandHostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"

// Only controller lifecycle responses are fixtures. Command parsing, Unix RPC,
// public adapter reuse, SSH files/key generation and editor argv are real paths.
type editorController struct {
	sync.Mutex
	environment core.Environment
	connection  core.ClientConnection
	created     []controlapi.EnvironmentCreateRequest
	prepared    []controlapi.EnvironmentSSHRequest
	deleted     []string
	failures    map[string]error
}

func editorCommandFixture(t *testing.T) (*editorController, string, string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("OpenSSH key generation unavailable")
	}
	home, workspace := t.TempDir(), filepath.Join(t.TempDir(), "project with spaces")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	socket := filepath.Join(t.TempDir(), "control.sock")
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	state := &editorController{failures: make(map[string]error)}
	server := control.NewServer()
	for _, method := range []string{controlapi.MethodEnvironmentStatus, controlapi.MethodEnvironmentCreate, controlapi.MethodEnvironmentDelete, controlapi.MethodEnvironmentSSH, controlapi.MethodEnvironmentConnections} {
		if err := server.Register(method, func(_ context.Context, raw json.RawMessage) (any, error) {
			state.Lock()
			defer state.Unlock()
			if err := state.failures[method]; err != nil {
				return nil, err
			}
			if method == controlapi.MethodEnvironmentCreate {
				var request controlapi.EnvironmentCreateRequest
				if err := json.Unmarshal(raw, &request); err != nil {
					return nil, err
				}
				state.created = append(state.created, request)
				state.environment = core.Environment{Name: request.Name, Workspace: core.Workspace{ID: "work", Path: request.WorkspacePath}, AccessMode: request.AccessMode, RuntimeRef: "owned-runtime"}
				return state.environment, nil
			}
			var request controlapi.EnvironmentNameRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			if state.environment.Name == "" || request.Environment != state.environment.Name {
				return nil, control.NewStatusError("not_found", "Environment absent")
			}
			switch method {
			case controlapi.MethodEnvironmentStatus:
				return core.EnvironmentStatus{Environment: state.environment, State: core.EnvironmentStopped}, nil
			case controlapi.MethodEnvironmentDelete:
				state.deleted = append(state.deleted, request.Environment)
				state.environment = core.Environment{}
				return nil, nil
			case controlapi.MethodEnvironmentSSH:
				var ssh controlapi.EnvironmentSSHRequest
				if err := json.Unmarshal(raw, &ssh); err != nil {
					return nil, err
				}
				state.prepared = append(state.prepared, ssh)
				state.connection = core.ClientConnection{ID: "ssh-one", Kind: "ssh", User: "root", TargetPort: 22, HostPublicKey: commandHostKey, Target: &core.StreamTarget{Environment: ssh.Environment, Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: state.environment.AccessMode, Service: "ssh", Grant: "ssh-one"}}
				return state.connection, nil
			case controlapi.MethodEnvironmentConnections:
				return []core.ClientConnection{state.connection}, nil
			}
			return nil, control.ErrInvalidArgument
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	return state, home, workspace
}

func editorFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEditorCommandCreatesReusesAndDeletesOnlyItsSSHEntry(t *testing.T) {
	state, home, workspace := editorCommandFixture(t)
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	personal := "Host personal\n  HostName personal.example\n"
	config := filepath.Join(home, ".ssh/config")
	if err := os.WriteFile(config, []byte(personal), 0600); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(t.TempDir(), "linked workspace")
	if err := os.Symlink(workspace, aliasPath); err != nil {
		t.Fatal(err)
	}
	name := defaultEnvironmentName(workspace)
	args := []string{"open", "--read-only", "--no-launch", aliasPath}
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	originalOut, originalArgs := os.Stdout, os.Args
	os.Stdout, os.Args = out, append([]string{"haco-vscode"}, args...)
	defer func() { os.Stdout, os.Args = originalOut, originalArgs }()
	Main()
	if got := string(editorFile(t, out.Name())); got != "SSH ready: haco-"+name+"\n" {
		t.Fatalf("unexpected command result: %q", got)
	}
	managed := filepath.Join(home, ".ssh/hacocoon", name+".conf")
	text := string(editorFile(t, managed))
	for _, required := range []string{"Host haco-" + name, "StrictHostKeyChecking yes", "IdentityFile ~/.ssh/hacocoon/identity", "ForwardAgent no"} {
		if !strings.Contains(text, required) {
			t.Fatalf("SSH config omitted %q", required)
		}
	}
	identityPath := filepath.Join(home, ".ssh/hacocoon/identity")
	identity := editorFile(t, identityPath)
	if !strings.Contains(string(identity), "OPENSSH PRIVATE KEY") {
		t.Fatal("client identity was not generated")
	}
	if err := runAdapter(context.Background(), args); err != nil {
		t.Fatal("reopen", err)
	}
	state.Lock()
	created, prepared := append([]controlapi.EnvironmentCreateRequest(nil), state.created...), append([]controlapi.EnvironmentSSHRequest(nil), state.prepared...)
	state.Unlock()
	if len(created) != 1 || created[0].Name != name || created[0].WorkspacePath != workspace || created[0].AccessMode != core.WorkspaceReadOnly || len(prepared) != 1 || !strings.HasPrefix(prepared[0].PublicKey, "ssh-ed25519 ") || strings.Contains(prepared[0].PublicKey, "PRIVATE") {
		t.Fatalf("create/reuse/public-key contract changed: create=%+v prepare count=%d", created, len(prepared))
	}
	if err := runAdapter(context.Background(), []string{"delete", "--name", name, workspace}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned SSH entry remained: %v", err)
	}
	if string(editorFile(t, identityPath)) != string(identity) || string(editorFile(t, config)) != "Include ~/.ssh/hacocoon/*.conf\n"+personal {
		t.Fatal("delete modified shared identity or personal configuration")
	}
	state.Lock()
	defer state.Unlock()
	if !reflect.DeepEqual(state.deleted, []string{name}) {
		t.Fatalf("deleted wrong Environment: %v", state.deleted)
	}
}

func TestEditorCommandLaunchesExactRemoteWorkspace(t *testing.T) {
	_, home, workspace := editorCommandFixture(t)
	bin := filepath.Join(home, "editor tools")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(home, "editor-argv")
	t.Setenv("EDITOR_TEST_ARGV", result)
	script := "#!/bin/sh\nif [ \"$1\" = --list-extensions ]; then\n  printf '%s\\n' ms-vscode-remote.remote-ssh\n  exit 0\nfi\nprintf '%s\\n' \"$@\" > \"$EDITOR_TEST_ARGV.tmp\"\nmv -- \"$EDITOR_TEST_ARGV.tmp\" \"$EDITOR_TEST_ARGV\"\n"
	if err := os.WriteFile(filepath.Join(bin, "code"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runAdapter(ctx, []string{"open", "--name", "editor-demo", workspace}); err != nil {
		t.Fatal(err)
	}
	for {
		data, err := os.ReadFile(result)
		if err == nil {
			if string(data) != "--folder-uri\nvscode-remote://ssh-remote+haco-editor-demo/workspace\n" {
				t.Fatalf("editor argv changed: %q", data)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("editor did not receive its Workspace URI")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestEditorCommandRefusesMismatchedOrFailedDeletion(t *testing.T) {
	for _, failure := range []string{"binding", "status", "delete"} {
		t.Run(failure, func(t *testing.T) {
			state, home, workspace := editorCommandFixture(t)
			if err := runAdapter(context.Background(), []string{"open", "--name", "owned", "--no-launch", workspace}); err != nil {
				t.Fatal(err)
			}
			managed := filepath.Join(home, ".ssh/hacocoon/owned.conf")
			before := editorFile(t, managed)
			state.Lock()
			switch failure {
			case "binding":
				state.environment.Workspace.Path = filepath.Join(workspace, "different")
			case "status":
				state.failures[controlapi.MethodEnvironmentStatus] = control.NewStatusError("unavailable", "status unavailable")
			case "delete":
				state.failures[controlapi.MethodEnvironmentDelete] = control.NewStatusError("recovery_required", "ownership retained")
			}
			state.Unlock()
			err := runAdapter(context.Background(), []string{"delete", "--name", "owned", workspace})
			if err == nil || (failure == "binding" && !strings.Contains(err.Error(), "workspace binding mismatch")) {
				t.Fatalf("unsafe delete was accepted: %v", err)
			}
			if string(editorFile(t, managed)) != string(before) {
				t.Fatal("failed delete discarded reconnectable SSH entry")
			}
			state.Lock()
			defer state.Unlock()
			if len(state.deleted) != 0 || state.environment.Name != "owned" {
				t.Fatal("failed delete released Environment ownership")
			}
		})
	}
}

func TestEditorCommandRefusesInvalidInputsAndFailedPreparation(t *testing.T) {
	state, home, workspace := editorCommandFixture(t)
	for _, args := range [][]string{nil, {"unknown"}, {"open"}, {"open", "--unknown", workspace}, {"open", workspace, workspace}, {"open", filepath.Join(workspace, "missing")}} {
		if err := runAdapter(context.Background(), args); err == nil {
			t.Fatalf("accepted invalid command %v", args)
		}
	}
	state.Lock()
	if len(state.created) != 0 {
		t.Error("invalid input reached creation")
	}
	state.failures[controlapi.MethodEnvironmentCreate] = control.NewStatusError("unavailable", "creation unavailable")
	state.Unlock()
	if err := runAdapter(context.Background(), []string{"open", "--name", "failed", workspace}); !errors.Is(err, clientadapter.ErrUnavailable) {
		t.Fatal("lost controller failure", err)
	}
	state.Lock()
	delete(state.failures, controlapi.MethodEnvironmentCreate)
	state.failures[controlapi.MethodEnvironmentSSH] = control.NewStatusError("incompatible_state", "SSH unavailable")
	state.Unlock()
	if err := runAdapter(context.Background(), []string{"open", "--name", "failed", workspace}); err == nil {
		t.Fatal("reported success without SSH preparation")
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh/hacocoon/failed.conf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published an unusable SSH entry: %v", err)
	}
}

func TestEditorCommandReportsDesktopAndLaunchFailures(t *testing.T) {
	for _, failure := range []string{"home", "stdout", "editor-removed"} {
		t.Run(failure, func(t *testing.T) {
			state, home, workspace := editorCommandFixture(t)
			var want error
			switch failure {
			case "home":
				t.Setenv("HOME", "")
			case "stdout":
				closed, err := os.CreateTemp(t.TempDir(), "closed-output")
				if err != nil {
					t.Fatal(err)
				}
				if err := closed.Close(); err != nil {
					t.Fatal(err)
				}
				original := os.Stdout
				os.Stdout = closed
				t.Cleanup(func() { os.Stdout = original })
				want = os.ErrClosed
			case "editor-removed":
				bin := filepath.Join(home, "bin")
				if err := os.Mkdir(bin, 0700); err != nil {
					t.Fatal(err)
				}
				// Model an editor being uninstalled between inspection and launch.
				if err := os.WriteFile(filepath.Join(bin, "code"), []byte("#!/bin/sh\nrm -- \"$0\"\nprintf '%s\\n' ms-vscode-remote.remote-ssh\n"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				want = os.ErrNotExist
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := runAdapter(ctx, []string{"open", "--name", "launch-failure", workspace})
			if err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatalf("lost %s failure: %v", failure, err)
			}
			state.Lock()
			defer state.Unlock()
			if failure == "home" {
				if len(state.created) != 0 || !strings.Contains(err.Error(), "$HOME") {
					t.Fatal("missing desktop home was not refused before creation", err)
				}
			} else if state.environment.Name != "launch-failure" || len(state.deleted) != 0 {
				t.Fatal("launch failure discarded the prepared Environment")
			} else if text := string(editorFile(t, filepath.Join(home, ".ssh/hacocoon/launch-failure.conf"))); !strings.Contains(text, "Host haco-launch-failure") {
				t.Fatal("launch failure discarded reconnectable SSH configuration")
			}
		})
	}
}
