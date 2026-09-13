package controlapi

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

func TestRunProcessEarlyExitDoesNotBecomeAnInputCopyFailure(t *testing.T) {
	for i := 0; i < 10; i++ {
		lifecycle := &processTestLifecycle{cleaned: make(chan error, 1)}
		lifecycle.execute = func(_ context.Context, _ io.Reader, _, _ io.Writer) (core.ExecutionResult, error) {
			return core.ExecutionResult{ExitCode: 17}, &control.SessionExitError{Code: 17}
		}
		client := processTestClient(t, lifecycle)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := client.RunStream(ctx, runapp.Spec{WorkspacePath: "/retained", Argv: []string{"exit-early"}}, false, bytes.NewReader(make([]byte, 1<<20)), io.Discard, io.Discard)
		cancel()
		if !result.CleanedUp || result.Execution.ExitCode != 17 || err == nil {
			t.Fatal("early exit lost to stdin teardown", result, err)
		}
		if err := <-lifecycle.cleaned; err != nil {
			t.Fatal(err)
		}
	}
}
