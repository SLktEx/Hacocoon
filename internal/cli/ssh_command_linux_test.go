//go:build linux

package cli

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

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

const desktopCommandHostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"

type desktopCommandController struct {
	sync.Mutex
	environment core.Environment
	listed      []core.Environment
	connection  core.ClientConnection
	prepared    []controlapi.EnvironmentSSHRequest
	calls       map[string]int
	failures    map[string]error
}

// Run the ordinary CLI against real Unix RPC, desktop key generation and owned
// SSH files. Only controller outcomes and external desktop executables are fixtures.
func desktopCommandFixture(t *testing.T, defaultWorkflow ...bool) (*desktopCommandController, string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("OpenSSH key generation unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	setCLITestLocale(t, "C")
	in, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	originalIn := os.Stdin
	os.Stdin = in
	t.Cleanup(func() { os.Stdin = originalIn; _ = in.Close() })
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh/config"), []byte("Host personal\n  HostName personal.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := &desktopCommandController{
		environment: core.Environment{Name: "dev", RuntimeRef: "owned-runtime", CreatedAt: time.Unix(100, 0), Workspace: core.Workspace{ID: "work", Path: "managed:work"}, AccessMode: core.WorkspaceReadWrite},
		calls:       make(map[string]int), failures: make(map[string]error),
	}
	state.listed = []core.Environment{state.environment}
	server := control.NewServer()
	if len(defaultWorkflow) != 0 && defaultWorkflow[0] {
		if err := server.Register(controlapi.MethodRepositoryManage, func(context.Context, json.RawMessage) (any, error) {
			return controlapi.RepositoryManageResponse{Sources: readySources("api", "web")}, nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := server.Register(controlapi.MethodWorkflow, func(_ context.Context, raw json.RawMessage) (any, error) {
			var req controlapi.WorkflowRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, err
			}
			state.Lock()
			defer state.Unlock()
			if req.Operation == "prepare" {
				return controlapi.WorkflowResponse{Reference: &workflow.Reference{Name: req.Prepare.Name, Workspace: state.environment.Workspace.ID}}, nil
			}
			if req.Operation == "open" {
				result := workflow.OpenResult{Reference: req.Open.Reference, Environment: state.environment}
				if defaultWorkflowFailure := state.failures[controlapi.MethodWorkflow]; defaultWorkflowFailure != nil {
					// Simulate replacement after preparation but before SSH setup.
					state.environment.RuntimeRef = "recycled-runtime"
				}
				return controlapi.WorkflowResponse{Open: &result}, nil
			}
			return nil, control.ErrInvalidArgument
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []string{controlapi.MethodEnvironmentList, controlapi.MethodEnvironmentStatus, controlapi.MethodEnvironmentSSH, controlapi.MethodEnvironmentConnections} {
		if err := server.Register(method, func(_ context.Context, raw json.RawMessage) (any, error) {
			state.Lock()
			defer state.Unlock()
			state.calls[method]++
			if err := state.failures[method]; err != nil {
				return nil, err
			}
			if method == controlapi.MethodEnvironmentList {
				return state.listed, nil
			}
			var request controlapi.EnvironmentNameRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			if request.Environment != state.environment.Name {
				return nil, control.NewStatusError("not_found", "Environment absent")
			}
			switch method {
			case controlapi.MethodEnvironmentStatus:
				return core.EnvironmentStatus{Environment: state.environment, State: core.EnvironmentStopped}, nil
			case controlapi.MethodEnvironmentConnections:
				return []core.ClientConnection{state.connection}, nil
			case controlapi.MethodEnvironmentSSH:
				var request controlapi.EnvironmentSSHRequest
				if err := json.Unmarshal(raw, &request); err != nil {
					return nil, err
				}
				state.prepared = append(state.prepared, request)
				state.connection = core.ClientConnection{ID: "ssh-one", Kind: "ssh", User: "root", TargetPort: 22, HostPublicKey: desktopCommandHostKey, Target: &core.StreamTarget{Environment: "dev", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}}
				return state.connection, nil
			}
			return nil, control.ErrInvalidArgument
		}); err != nil {
			t.Fatal(err)
		}
	}
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	return state, home
}

func desktopCommandFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSSHCommandSetupReuseAndCleanup(t *testing.T) {
	state, home := desktopCommandFixture(t)
	for _, args := range [][]string{{"ssh", "setup", "dev"}, {"ssh", "setup"}} {
		code, out, diagnostic := captureRun(t, args...)
		if code != 0 || out != "SSH ready: haco-dev\n" || !strings.Contains(diagnostic, "[succeeded] ssh_connection") {
			t.Fatal(args, code, out, diagnostic)
		}
	}
	managed := filepath.Join(home, ".ssh/hacocoon/dev.conf")
	config := desktopCommandFile(t, managed)
	for _, setting := range []string{"Host haco-dev", "StrictHostKeyChecking yes", "ForwardAgent no", "ProxyCommand", "IdentityFile ~/.ssh/hacocoon/identity"} {
		if !strings.Contains(string(config), setting) {
			t.Fatal("managed SSH contract lost", setting)
		}
	}
	identity := desktopCommandFile(t, filepath.Join(home, ".ssh/hacocoon/identity"))
	if !strings.Contains(string(identity), "OPENSSH PRIVATE KEY") {
		t.Fatal("desktop identity was not generated")
	}
	state.Lock()
	if len(state.prepared) != 1 || !strings.HasPrefix(state.prepared[0].PublicKey, "ssh-ed25519 ") || strings.Contains(state.prepared[0].PublicKey, "PRIVATE") || state.calls[controlapi.MethodEnvironmentConnections] != 1 {
		t.Error("reuse replaced the grant or sent a private key", state.prepared, state.calls)
	}
	state.Unlock()
	if code, _, diagnostic := captureRun(t, "ssh", "cleanup"); code != 0 || diagnostic != "" || !reflect.DeepEqual(desktopCommandFile(t, managed), config) {
		t.Fatal("cleanup removed a live target", code, diagnostic)
	}
	state.Lock()
	state.failures[controlapi.MethodEnvironmentStatus] = control.NewStatusError("recovery_required", "ownership unresolved")
	state.Unlock()
	if code, _, diagnostic := captureRun(t, "ssh", "cleanup"); code != 1 || !strings.Contains(diagnostic, "could not establish stale targets") || !reflect.DeepEqual(desktopCommandFile(t, managed), config) {
		t.Fatal("uncertain cleanup changed configuration", code, diagnostic)
	}
	state.Lock()
	state.failures[controlapi.MethodEnvironmentStatus] = control.NewStatusError("not_found", "Environment absent")
	state.Unlock()
	if code, _, diagnostic := captureRun(t, "ssh", "cleanup"); code != 0 || diagnostic != "" {
		t.Fatal("confirmed stale cleanup failed", code, diagnostic)
	}
	if _, err := os.Stat(managed); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale entry remains", err)
	}
	if !reflect.DeepEqual(desktopCommandFile(t, filepath.Join(home, ".ssh/hacocoon/identity")), identity) || string(desktopCommandFile(t, filepath.Join(home, ".ssh/config"))) != "Include ~/.ssh/hacocoon/*.conf\nHost personal\n  HostName personal.example\n" {
		t.Fatal("cleanup changed shared credentials or personal config")
	}
}

func TestSSHCommandFailureStopsBeforeLaunch(t *testing.T) {
	for _, mode := range []string{"list-fails", "none", "multiple", "changed-selection", "desktop", "prepare"} {
		t.Run(mode, func(t *testing.T) {
			state, home := desktopCommandFixture(t)
			state.Lock()
			switch mode {
			case "list-fails":
				state.failures[controlapi.MethodEnvironmentList] = control.NewStatusError("recovery_required", "catalog unavailable")
			case "none":
				state.listed = nil
			case "multiple":
				state.listed = append(state.listed, core.Environment{Name: "another"})
			case "changed-selection":
				state.environment.RuntimeRef = "replacement"
			case "prepare":
				state.failures[controlapi.MethodEnvironmentSSH] = control.NewStatusError("recovery_required", "retained preparation")
			}
			state.Unlock()
			if mode == "desktop" {
				t.Setenv("HOME", "")
			}
			if _, err := os.Stdin.WriteString("payload for a later command"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stdin.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			code, out, diagnostic := captureRun(t, "ssh", "setup")
			wantCode := 1
			if mode == "none" || mode == "multiple" {
				wantCode = 2
			}
			if code != wantCode || out != "" || diagnostic == "" || strings.Contains(diagnostic, "[succeeded] ssh_connection") {
				t.Fatal("failed setup reported readiness", code, out, diagnostic)
			}
			if position, err := os.Stdin.Seek(0, 1); err != nil || position != 0 {
				t.Fatal("noninteractive choice consumed stdin", position, err)
			}
			if _, err := os.Stat(filepath.Join(home, ".ssh/hacocoon/dev.conf")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed setup published a target", err)
			}
			state.Lock()
			defer state.Unlock()
			if len(state.prepared) != 0 || (mode != "prepare" && state.calls[controlapi.MethodEnvironmentSSH] != 0) {
				t.Fatal("unverified selection granted SSH", state.calls)
			}
		})
	}
}

func TestSSHCommandRejectsLostResult(t *testing.T) {
	state, home := desktopCommandFixture(t)
	output, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	diagnostic, err := os.CreateTemp(t.TempDir(), "diagnostic")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = diagnostic.Close() }()
	originalOut, originalError := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = output, diagnostic
	defer func() { os.Stdout, os.Stderr = originalOut, originalError }()
	if code := runSSH([]string{"setup", "dev"}); code != 1 {
		t.Fatal("failed result write reported success", code)
	}
	if text := string(desktopCommandFile(t, diagnostic.Name())); !strings.Contains(text, cliMessage("error.write_result")) {
		t.Fatal("missing write failure diagnostic", text)
	}
	if text := string(desktopCommandFile(t, filepath.Join(home, ".ssh/hacocoon/dev.conf"))); !strings.Contains(text, "Host haco-dev") {
		t.Fatal("result failure discarded prepared access")
	}
	state.Lock()
	defer state.Unlock()
	if len(state.prepared) != 1 {
		t.Fatal("result failure repeated preparation", state.prepared)
	}
}

func TestOpenCommandLaunchAndRetainedSSH(t *testing.T) {
	for _, mode := range []string{"ssh", "ssh-fails", "editor", "editor-fails", "selection"} {
		t.Run(mode, func(t *testing.T) {
			_, home := desktopCommandFixture(t)
			bin := filepath.Join(home, "desktop tools")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			result := filepath.Join(home, "launched-argv")
			t.Setenv("SSH_CLI_TEST_ARGS", result)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SSH_CLI_TEST_ARGS.tmp\"\nmv -- \"$SSH_CLI_TEST_ARGS.tmp\" \"$SSH_CLI_TEST_ARGS\"\n"
			name, args := "ssh", []string{"open", "--client", "ssh", "dev"}
			if mode == "selection" {
				args = []string{"open", "--select", "--client", "ssh"}
			}
			want := "-t\nhaco-dev\ncd /workspace && exec bash -l\n"
			if strings.HasPrefix(mode, "editor") {
				name, args = "code", []string{"open", "dev"}
				want = "--folder-uri\nvscode-remote://ssh-remote+haco-dev/workspace\n"
				script = "#!/bin/sh\nif [ \"$1\" = --list-extensions ]; then\n  printf '%s\\n' ms-vscode-remote.remote-ssh\n  exit 0\nfi\n" + strings.TrimPrefix(script, "#!/bin/sh\n")
			}
			switch mode {
			case "editor-fails":
				script = "#!/bin/sh\nexit 19\n"
			case "ssh-fails":
				script += "exit 17\n"
			}
			if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			code, out, diagnostic := captureRun(t, args...)
			failed := strings.HasSuffix(mode, "fails")
			if (failed && code != 1) || (!failed && code != 0) || out != "SSH ready: haco-dev\n" {
				t.Fatal("wrong launch outcome", code, out, diagnostic)
			}
			if failed && strings.Contains(diagnostic, "Editor process launched") {
				t.Fatal("failed launch reported success", diagnostic)
			}
			if mode == "editor-fails" {
				if !strings.Contains(diagnostic, "SSH preparation remains available") {
					t.Fatal("retained setup not explained", diagnostic)
				}
			} else {
				deadline := time.Now().Add(5 * time.Second)
				for {
					data, err := os.ReadFile(result)
					if err == nil {
						if string(data) != want {
							t.Fatal("desktop command changed target", string(data))
						}
						break
					}
					if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
						t.Fatal("desktop did not start", err)
					}
					time.Sleep(time.Millisecond)
				}
			}
			if text := string(desktopCommandFile(t, filepath.Join(home, ".ssh/hacocoon/dev.conf"))); !strings.Contains(text, "Host haco-dev") {
				t.Fatal("launch discarded prepared SSH")
			}
		})
	}
}

func TestDefaultOpenLaunchAndRecycledOwnerRefusal(t *testing.T) {
	for _, replaced := range []bool{false, true} {
		t.Run(map[bool]string{false: "launch", true: "replaced"}[replaced], func(t *testing.T) {
			state, home := desktopCommandFixture(t, true)
			bin := filepath.Join(home, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(home, "launched")
			t.Setenv("DEFAULT_OPEN_MARKER", marker)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DEFAULT_OPEN_MARKER\"\n"
			if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			if replaced {
				state.failures[controlapi.MethodWorkflow] = core.ErrCapabilityStale
			}
			code, _, diagnostic := captureRun(t, "open", "--client", "ssh")
			if replaced {
				if code != 1 {
					t.Fatal("recycled Env accepted", code, diagnostic)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("launched replaced Env", err)
				}
				state.Lock()
				defer state.Unlock()
				if len(state.prepared) != 0 {
					t.Fatal("granted access to replaced Env")
				}
				return
			}
			if code != 0 {
				t.Fatal(code, diagnostic)
			}
			if raw := desktopCommandFile(t, marker); !strings.Contains(string(raw), "haco-dev") {
				t.Fatal("wrong shell target", string(raw))
			}
		})
	}
}
