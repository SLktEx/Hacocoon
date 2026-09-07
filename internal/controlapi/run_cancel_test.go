package controlapi

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

type interruptedRunLifecycle struct {
	started chan struct{}
	cleaned chan error
}

func (r *interruptedRunLifecycle) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return core.Environment{Name: spec.Name}, nil
}
func (r *interruptedRunLifecycle) Exec(ctx context.Context, _ string, _ core.ExecutionRequest) (core.ExecutionResult, error) {
	close(r.started)
	<-ctx.Done()
	return core.ExecutionResult{}, ctx.Err()
}
func (r *interruptedRunLifecycle) Delete(ctx context.Context, _ string) error {
	err := ctx.Err()
	if _, ok := ctx.Deadline(); !ok {
		err = errors.New("cleanup has no deadline")
	}
	r.cleaned <- err
	return err
}
func TestRunClientCancellationCleansUpOnController(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := &interruptedRunLifecycle{started: make(chan struct{}), cleaned: make(chan error, 1)}
	server := control.NewServer()
	if err := RegisterGeneral(server, fakeBases{}, runapp.New(lifecycle), fakeEvents{}, &fakeCapabilities{}); err != nil {
		t.Fatal(err)
	}
	serverCtx, stopServer := context.WithCancel(context.Background())
	defer stopServer()
	go func() { _ = server.Serve(serverCtx, listener) }()
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := client.Run(ctx, runapp.Spec{WorkspacePath: "/work", Argv: []string{"sleep", "600"}})
		result <- err
	}()
	select {
	case <-lifecycle.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not start")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled caller reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("caller did not stop")
	}
	select {
	case err := <-lifecycle.cleaned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client disconnection did not trigger controller cleanup")
	}
}

func TestRunStreamRejectsUnexpectedInputAndCleansUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := &interruptedRunLifecycle{started: make(chan struct{}), cleaned: make(chan error, 1)}
	server := control.NewServer()
	if err := RegisterGeneral(server, fakeBases{}, runapp.New(lifecycle), fakeEvents{}, &fakeCapabilities{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Serve(ctx, listener) }()
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := client.wire.OpenStream(ctx, MethodRun, runapp.Spec{WorkspacePath: "/work", Argv: []string{"sleep", "600"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case <-lifecycle.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not start")
	}
	if _, err := conn.Write([]byte("unexpected")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-lifecycle.cleaned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("unexpected input did not trigger cleanup")
	}
}
