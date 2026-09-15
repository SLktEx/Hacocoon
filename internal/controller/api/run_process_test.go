package controlapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
)

type processTestLifecycle struct {
	instance   string
	created    atomic.Int32
	cleanupErr error
	cleaned    chan error
	execute    func(context.Context, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

func (r *processTestLifecycle) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	r.instance = spec.EphemeralInstance
	r.created.Add(1)
	return core.Environment{Name: spec.Name}, nil
}
func (r *processTestLifecycle) Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, errors.New("stream used captured execution")
}
func (r *processTestLifecycle) ExecRunStream(ctx context.Context, _, instance string, request core.ProcessRequest, in io.Reader, out, diagnostic io.Writer) (core.ExecutionResult, error) {
	if instance != r.instance || !core.ValidEnvironmentInstanceID(instance) || request.WorkingDirectory != "/workspace" {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	return r.execute(ctx, in, out, diagnostic)
}
func (r *processTestLifecycle) DeleteRun(ctx context.Context, _, instance string) error {
	err := ctx.Err()
	if _, ok := ctx.Deadline(); !ok || instance != r.instance {
		err = core.ErrCapabilityStale
	}
	r.cleaned <- err
	return errors.Join(err, r.cleanupErr)
}

func processTestClient(t *testing.T, lifecycle *processTestLifecycle) *Client {
	t.Helper()
	server := control.NewServer()
	if err := RegisterGeneral(server, fakeBases{}, runapp.New(lifecycle), fakeEvents{}, &fakeCapabilities{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRunProcessEOFExitAndCanonicalCleanupReceipt(t *testing.T) {
	for _, cleanupFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "cleaned", true: "recovery-required"}[cleanupFails], func(t *testing.T) {
			data := bytes.Repeat([]byte("streamed\x00input\n"), 100000)
			lifecycle := &processTestLifecycle{cleaned: make(chan error, 1)}
			if cleanupFails {
				lifecycle.cleanupErr = core.ErrRecoveryRequired
			}
			lifecycle.execute = func(ctx context.Context, in io.Reader, out, diagnostic io.Writer) (core.ExecutionResult, error) {
				if _, err := io.Copy(out, in); err != nil {
					return core.ExecutionResult{}, err
				}
				if ctx.Err() != nil {
					return core.ExecutionResult{}, ctx.Err()
				}
				_, err := io.WriteString(diagnostic, "separate stderr")
				if err != nil {
					return core.ExecutionResult{}, err
				}
				return core.ExecutionResult{ExitCode: 17}, &control.SessionExitError{Code: 17}
			}
			client := processTestClient(t, lifecycle)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var out, diagnostic bytes.Buffer
			result, err := client.RunStream(ctx, runapp.Spec{WorkspacePath: "/retained", Argv: []string{"cat"}}, false, bytes.NewReader(data), &out, &diagnostic)
			if err == nil || result.CleanedUp == cleanupFails || result.Execution.ExitCode != 17 || !bytes.Equal(out.Bytes(), data) || diagnostic.String() != "separate stderr" {
				t.Fatal("lost process or cleanup result", result, err)
			}
			if result.Execution.StdoutBytes != int64(len(data)) || result.Execution.StderrBytes != int64(diagnostic.Len()) || result.Execution.Stdout != "" || result.Execution.Stderr != "" {
				t.Fatal("stream was recaptured or counted incorrectly", result.Execution)
			}
			if err := <-lifecycle.cleaned; err != nil {
				t.Fatal("cleanup lost ownership/context", err)
			}
		})
	}
}

func TestRunProcessDisconnectWhileInputBlockedStillCleansUp(t *testing.T) {
	started := make(chan struct{})
	lifecycle := &processTestLifecycle{cleaned: make(chan error, 1)}
	lifecycle.execute = func(ctx context.Context, _ io.Reader, _, _ io.Writer) (core.ExecutionResult, error) {
		close(started)
		<-ctx.Done()
		return core.ExecutionResult{}, ctx.Err()
	}
	client := processTestClient(t, lifecycle)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := client.RunStream(ctx, runapp.Spec{WorkspacePath: "/retained", Argv: []string{"ignores-input"}}, false, bytes.NewReader(make([]byte, 1<<20)), io.Discard, io.Discard)
		if result.CleanedUp {
			err = errors.New("disconnected client confirmed cleanup")
		}
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disconnect succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client stayed blocked")
	}
	select {
	case err := <-lifecycle.cleaned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("controller did not clean up")
	}
}

func TestRunRequestsRejectInvalidInputBeforeCreation(t *testing.T) {
	lifecycle := &processTestLifecycle{cleaned: make(chan error, 1)}
	client := processTestClient(t, lifecycle)
	for _, test := range []struct {
		name, method string
		request      any
	}{
		{"missing-dimensions", MethodRunStream, RunStreamRequest{Spec: runapp.Spec{Argv: []string{"bash"}}, TTY: true}},
		{"terminal-without-tty", MethodRunStream, RunStreamRequest{Spec: runapp.Spec{Argv: []string{"cat"}}, Terminal: TerminalMetadata{Term: "xterm"}}},
		{"nul-command", MethodRunStream, RunStreamRequest{Spec: runapp.Spec{Argv: []string{"bad\x00command"}}}},
		{"oversized", MethodRunStream, RunStreamRequest{Spec: runapp.Spec{Argv: []string{strings.Repeat("x", 64<<10)}}}},
		{"wrong-json-type", MethodRunStream, "not a request object"},
		{"unknown-field", MethodRunStream, map[string]any{"spec": runapp.Spec{Argv: []string{"true"}}, "unreviewed": true}},
		{"invalid-terminal-identity", MethodRunStream, RunStreamRequest{Spec: runapp.Spec{Argv: []string{"bash"}}, TTY: true, Terminal: TerminalMetadata{Term: "xterm\ncommand", Columns: 80, Rows: 24}}},
		{"missing-argv", MethodRun, runapp.Spec{WorkspacePath: "/retained"}},
		{"captured-wrong-json-type", MethodRun, "not a request object"},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, err := client.wire.OpenSession(context.Background(), test.method, test.request)
			if conn != nil {
				_ = conn.Close()
			}
			var status *control.StatusError
			if !errors.As(err, &status) || status.Code != "invalid_argument" || lifecycle.created.Load() != 0 {
				t.Fatal("invalid request was not refused before Environment creation", err)
			}
		})
	}
	lifecycle.execute = func(context.Context, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error) {
		return core.ExecutionResult{}, nil
	}
	if _, err := client.RunStream(context.Background(), runapp.Spec{WorkspacePath: "/retained", Argv: []string{"true"}}, false, strings.NewReader(""), io.Discard, io.Discard); err != nil || lifecycle.created.Load() != 1 {
		t.Fatal("refused input broke subsequent valid execution", err)
	}
	if err := <-lifecycle.cleaned; err != nil {
		t.Fatal("valid execution did not finish canonical cleanup", err)
	}
}
