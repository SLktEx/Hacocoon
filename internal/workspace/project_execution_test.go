package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
	"time"
)

func TestWorkspaceBoundOperationsRefuseRecycledName(t *testing.T) {
	store := resumableStore()
	runtime := &startingRuntime{start: func(context.Context, string) error { t.Fatal("started another Workspace"); return nil }}
	service := New(runtime, store)
	if err := service.StartForWorkspace(context.Background(), "resume", "old"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	if _, err := service.ExecForWorkspace(context.Background(), "resume", "old", core.ExecutionRequest{Argv: []string{"true"}}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	if _, err := service.ExecForWorkspace(context.Background(), "resume", "", core.ExecutionRequest{Argv: []string{"true"}}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}

type guardedExecutionRuntime struct {
	fakeEnvironmentRuntime
	entered chan struct{}
	release chan struct{}
}

func (r *guardedExecutionRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	close(r.entered)
	<-r.release
	return core.ExecutionResult{}, nil
}
func TestWorkspaceExecutionHoldsLifecycleLock(t *testing.T) {
	store := resumableStore()
	runtime := &guardedExecutionRuntime{entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := New(runtime, store).ExecForWorkspace(context.Background(), "resume", "work", core.ExecutionRequest{Argv: []string{"true"}})
		done <- err
	}()
	<-runtime.entered
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	err := New(runtime, store).Delete(ctx, "resume")
	cancel()
	close(runtime.release)
	if runErr := <-done; runErr != nil {
		t.Fatal(runErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("inverse operation overtook execution: %v", err)
	}
}
