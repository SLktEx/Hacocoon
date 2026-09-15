//go:build linux

package hostcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// This fixture supplies controller responses over the shipped Unix transport.
// It checks CLI serialization and presentation, not provider lifecycle acceptance.
func hostCommandClient(t *testing.T, configure func(*control.Server)) *controlapi.Client {
	t.Helper()
	server := control.NewServer()
	configure(server)
	socket := filepath.Join(t.TempDir(), "host.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	t.Setenv("HACO_UI_LANGUAGE", "en")
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func captureHostCommand(t *testing.T, run func() error) (string, string, error) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	diagnostic, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = diagnostic.Close() }()
	originalOut, originalError := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, diagnostic
	defer func() { os.Stdout, os.Stderr = originalOut, originalError }()
	commandErr := run()
	read := func(file *os.File) string {
		t.Helper()
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read(out), read(diagnostic), commandErr
}

func TestHostCommandsPreserveControllerResultsAndMachineOutput(t *testing.T) {
	environment := core.Environment{
		Name: "demo", Workspace: core.Workspace{ID: "workspace:demo", Path: "/work/project with spaces"},
		AccessMode: core.WorkspaceReadOnly, RuntimeRef: "haco-demo",
		Base: &core.BaseRef{Name: "development", Revision: "revision-7"},
		Resources: core.ResourceBudget{CPU: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 2},
			MemoryBytes: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 4 << 30},
			PIDs:        core.ResourceLimit{Mode: core.ResourceLimitUnlimited}},
	}
	status := core.EnvironmentStatus{Environment: environment, State: core.EnvironmentStopped}
	created := make(chan controlapi.EnvironmentCreateRequest, 1)
	deleted := make(chan string, 1)
	client := hostCommandClient(t, func(server *control.Server) {
		handlers := map[string]control.Handler{
			controlapi.MethodPing: func(context.Context, json.RawMessage) (any, error) {
				return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
			},
			controlapi.MethodEnvironmentCreate: func(_ context.Context, raw json.RawMessage) (any, error) {
				var request controlapi.EnvironmentCreateRequest
				if err := json.Unmarshal(raw, &request); err != nil {
					return nil, err
				}
				created <- request
				return environment, nil
			},
			controlapi.MethodEnvironmentList: func(context.Context, json.RawMessage) (any, error) {
				return []core.Environment{environment}, nil
			},
			controlapi.MethodEnvironmentStatus: func(_ context.Context, raw json.RawMessage) (any, error) {
				var request controlapi.EnvironmentNameRequest
				if err := json.Unmarshal(raw, &request); err != nil || request.Environment != environment.Name {
					return nil, control.ErrInvalidArgument
				}
				return status, nil
			},
			controlapi.MethodEnvironmentDelete: func(_ context.Context, raw json.RawMessage) (any, error) {
				var request controlapi.EnvironmentNameRequest
				if err := json.Unmarshal(raw, &request); err != nil {
					return nil, err
				}
				deleted <- request.Environment
				return nil, nil
			},
		}
		for method, handler := range handlers {
			if err := server.Register(method, handler); err != nil {
				t.Fatal(err)
			}
		}
	})
	invoke := func(args ...string) string {
		t.Helper()
		out, diagnostic, err := captureHostCommand(t, func() error { return dispatch(context.Background(), client, args) })
		if err != nil || diagnostic != "" {
			t.Fatalf("%v: error=%v stderr=%q", args, err, diagnostic)
		}
		return out
	}
	if out := invoke("env", "create", "--workspace", environment.Workspace.Path, "--read-only", "--base", "development", "--cpu", "2", "--memory", "4GiB", "--pids", "unlimited", "demo"); out != "demo\t/work/project with spaces\tro\n" {
		t.Fatal(out)
	}
	request := <-created
	if request.Name != environment.Name || request.WorkspacePath != environment.Workspace.Path || request.AccessMode != environment.AccessMode || request.Base != environment.Base.Name || request.Resources != environment.Resources {
		t.Fatalf("creation options changed in transit: %#v", request)
	}
	if out := invoke("env", "list"); out != "demo\tro\t/work/project with spaces\n" {
		t.Fatal(out)
	}
	var listed []core.Environment
	if err := json.Unmarshal([]byte(invoke("env", "list", "--json")), &listed); err != nil || !reflect.DeepEqual(listed, []core.Environment{environment}) {
		t.Fatal("list JSON changed controller data", listed, err)
	}
	var decoded core.EnvironmentStatus
	if err := json.Unmarshal([]byte(invoke("env", "status", "demo", "--json")), &decoded); err != nil || !reflect.DeepEqual(decoded, status) {
		t.Fatal("status JSON changed controller data", decoded, err)
	}
	wantStatus := "name: demo\nstate: stopped\nruntime: haco-demo\nworkspace: /work/project with spaces\naccess: ro\nbase: development\nbase-revision: revision-7\ncpu: 2\nmemory-bytes: 4294967296\npids: unlimited\nroot-bytes: unlimited\n"
	if out := invoke("env", "status", "demo"); out != wantStatus {
		t.Fatalf("status: %q", out)
	}
	if out := invoke("env", "delete", "demo"); out != "" || <-deleted != "demo" {
		t.Fatal("delete output or target changed", out)
	}
	if out := invoke("doctor"); !strings.Contains(out, "controller: "+control.SocketPath()+"\n") || !strings.Contains(out, "protocol-version: 1\n") {
		t.Fatal("doctor did not report its controller", out)
	}
	// Main must use the configured controller endpoint after the entry moved.
	oldArgs := os.Args
	os.Args = []string{"haco-host", "doctor"}
	defer func() { os.Args = oldArgs }()
	out, diagnostic, err := captureHostCommand(t, func() error { Main(); return nil })
	if err != nil || diagnostic != "" || !strings.Contains(out, "Hacocoon logical Host client\n") {
		t.Fatal(out, diagnostic, err)
	}
	os.Args = []string{"haco-host", "--help"}
	out, diagnostic, err = captureHostCommand(t, func() error { Main(); return nil })
	if err != nil || diagnostic != "" || !strings.Contains(out, "haco-host env list") {
		t.Fatal(out, diagnostic, err)
	}
}

func TestHostExecPreservesLiteralArgumentsStreamsAndExitStatus(t *testing.T) {
	client := hostCommandClient(t, func(server *control.Server) {
		if err := server.Register(controlapi.MethodEnvironmentExec, func(ctx context.Context, raw json.RawMessage) (any, error) {
			var request controlapi.EnvironmentExecRequest
			if err := json.Unmarshal(raw, &request); err != nil || request.Environment != "demo" || len(request.Argv) == 0 {
				return nil, control.ErrInvalidArgument
			}
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, request.Argv[0], request.Argv[1:]...)
			var stdout, stderr bytes.Buffer
			process.Stdout, process.Stderr = &stdout, &stderr
			err := process.Run()
			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				return nil, err
			}
			return core.ExecutionResult{ExitCode: process.ProcessState.ExitCode(), Stdout: stdout.String(), Stderr: stderr.String()}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	for _, exit := range []string{"0", "17"} {
		args := []string{"env", "exec", "demo", "--", "/bin/sh", "-c", `printf '%s\n' "$2" "$3"; printf 'diagnostic\n' >&2; exit "$1"`, "fixture", exit, "$(do-not-run); *", "--help"}
		out, diagnostic, err := captureHostCommand(t, func() error { return dispatch(context.Background(), client, args) })
		if out != "$(do-not-run); *\n--help\n" || diagnostic != "diagnostic\n" {
			t.Fatal("exec altered arguments or mixed streams", out, diagnostic, err)
		}
		if exit == "0" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			var status interface{ ExitCode() int }
			if !errors.As(err, &status) || status.ExitCode() != 17 || err.Error() != "command exited 17" {
				t.Fatal("remote exit status was lost", err)
			}
		}
	}
}

func TestHostControllerFailuresNeverReportSuccess(t *testing.T) {
	client := hostCommandClient(t, func(server *control.Server) {
		for _, method := range []string{controlapi.MethodPing, controlapi.MethodEnvironmentCreate, controlapi.MethodEnvironmentList, controlapi.MethodEnvironmentStatus, controlapi.MethodEnvironmentDelete, controlapi.MethodEnvironmentExec} {
			if err := server.Register(method, func(context.Context, json.RawMessage) (any, error) {
				return nil, control.NewStatusError("unavailable", "operation did not complete")
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := server.RegisterStream(controlapi.MethodEnvironmentShell, func(context.Context, json.RawMessage) (control.Stream, error) {
			return nil, control.NewStatusError("unavailable", "operation did not complete")
		}); err != nil {
			t.Fatal(err)
		}
	})
	for _, args := range [][]string{{"doctor"}, {"env", "create", "--workspace", "/work", "demo"}, {"env", "list"}, {"env", "status", "demo"}, {"env", "delete", "demo"}, {"env", "exec", "demo", "--", "true"}, {"env", "shell", "demo"}} {
		out, diagnostic, err := captureHostCommand(t, func() error { return dispatch(context.Background(), client, args) })
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "unavailable" || out != "" || diagnostic != "" {
			t.Fatalf("%v: out=%q stderr=%q error=%v", args, out, diagnostic, err)
		}
	}
}

func TestHostShellTransfersBytesAndClosesOnCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	finished := make(chan error, 1)
	client := hostCommandClient(t, func(server *control.Server) {
		if err := server.RegisterStream(controlapi.MethodEnvironmentShell, func(_ context.Context, raw json.RawMessage) (control.Stream, error) {
			var request controlapi.EnvironmentShellRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			return func(_ context.Context, conn net.Conn) error {
				if request.Environment == "cancel" {
					started <- struct{}{}
					_, err := io.Copy(io.Discard, conn)
					finished <- err
					return err
				}
				buffer := make([]byte, 4)
				if _, err := io.ReadFull(conn, buffer); err != nil {
					return err
				}
				_, err := conn.Write(append([]byte("reply:"), buffer...))
				return err
			}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	var out bytes.Buffer
	if err := envShellCommand(context.Background(), client, []string{"demo"}, bytes.NewReader([]byte{0, 255, 13, 10}), &out); err != nil || !bytes.Equal(out.Bytes(), []byte{'r', 'e', 'p', 'l', 'y', ':', 0, 255, 13, 10}) {
		t.Fatal("shell changed bytes", out.Bytes(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	input, writer := io.Pipe()
	defer func() { _ = input.Close() }()
	defer func() { _ = writer.Close() }()
	go func() { done <- envShellCommand(ctx, client, []string{"cancel"}, input, io.Discard) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("shell did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shell ignored cancellation")
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled shell kept controller stream open")
	}
}

func TestHostInvalidCommandsNeverReachController(t *testing.T) {
	// An absent endpoint distinguishes local usage refusal from an RPC failure.
	client, err := controlapi.NewClient(filepath.Join(t.TempDir(), "absent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"env"}, {"env", "unknown"}, {"doctor", "extra"}, {"env", "list", "extra"}, {"env", "status"}, {"env", "status", "demo", "bad"}, {"env", "exec", "demo", "true"}, {"env", "shell"}, {"env", "delete", "demo", "other"}, {"env", "create", "demo"}} {
		_, _, err := captureHostCommand(t, func() error { return dispatch(context.Background(), client, args) })
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	_, _, err = captureHostCommand(t, func() error { return dispatch(context.Background(), nil, []string{"doctor"}) })
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
