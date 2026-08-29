package run

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type blockingCleanupEnvironment struct{}

func (blockingCleanupEnvironment) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return core.Environment{Name: spec.Name}, nil
}

func (blockingCleanupEnvironment) Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (blockingCleanupEnvironment) Delete(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestRunCleanupUsesIndependentBoundedDeadline(t *testing.T) {
	service := New(blockingCleanupEnvironment{})
	service.newName = func() (string, error) { return "run-timeout", nil }
	service.cleanupTimeout = 20 * time.Millisecond

	caller, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	result, err := service.Run(caller, Spec{WorkspacePath: "/work/demo", Argv: []string{"true"}})
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
	if result.CleanedUp {
		t.Fatal("timed-out cleanup reported success")
	}
	if elapsed < 10*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("cleanup was not independently bounded: %s", elapsed)
	}
}
