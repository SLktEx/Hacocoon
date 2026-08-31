package controlapi

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

type fakeWireExitError struct{ code int }

func (e fakeWireExitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e fakeWireExitError) ExitCode() int { return e.code }

type fakeExitRunner struct{}

func (fakeExitRunner) Run(context.Context, runapp.Spec) (runapp.Result, error) {
	return runapp.Result{
		Environment: "run-exit",
		Execution: runapp.ExecutionResult{
			ExitCode: 17,
			Stderr:   "guest failed\n",
		},
		CleanedUp: true,
	}, fakeWireExitError{code: 17}
}

func TestGeneralControllerPreservesProcessExitCodeAcrossWire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := RegisterGeneral(server, fakeBases{}, fakeExitRunner{}, fakeEvents{}, &fakeCapabilities{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not stop")
		}
	}()

	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Run(context.Background(), runapp.Spec{WorkspacePath: "/work", Argv: []string{"false"}})
	if result.Execution.ExitCode != 17 || result.Execution.Stderr != "guest failed\n" || !result.CleanedUp {
		t.Fatalf("result = %#v", result)
	}
	var exitCoder interface{ ExitCode() int }
	if !errors.As(err, &exitCoder) || exitCoder.ExitCode() != 17 {
		t.Fatalf("error = %v, want remote exit code 17", err)
	}
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "internal" {
		t.Fatalf("error = %v, want wrapped internal status", err)
	}
}
