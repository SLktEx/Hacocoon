package workspace

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type processRuntimeFixture struct {
	fakeEnvironmentRuntime
	started chan struct{}
	finish  chan struct{}
}

func (r *processRuntimeFixture) ExecEnvironmentStream(ctx context.Context, ref string, request core.ProcessRequest, _ io.Reader, _, _ io.Writer) (core.ExecutionResult, error) {
	if ref != "owned-process" || request.WorkingDirectory != "/workspace" {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	close(r.started)
	select {
	case <-r.finish:
		return core.ExecutionResult{}, nil
	case <-ctx.Done():
		return core.ExecutionResult{}, ctx.Err()
	}
}

func TestRunProcessPinsCreationThroughConcurrentDeletion(t *testing.T) {
	ctx := context.Background()
	catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	id, _ := core.NewEnvironmentInstanceID()
	marker := core.EphemeralRun{InstanceID: id, EnvironmentID: "run-process", State: core.EphemeralRunCreating, CreatedAt: time.Now().UTC()}
	if err := catalog.PutEphemeralRun(ctx, marker); err != nil {
		t.Fatal(err)
	}
	runtime := &processRuntimeFixture{fakeEnvironmentRuntime: fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "owned-process"}}, started: make(chan struct{}), finish: make(chan struct{})}
	service := New(runtime, catalog)
	env, err := service.Create(ctx, core.EnvironmentSpec{Name: marker.EnvironmentID, WorkspacePath: t.TempDir(), EphemeralInstance: id})
	if err != nil {
		t.Fatal(err)
	}
	request := core.ProcessRequest{WorkingDirectory: "/workspace", Argv: []string{"cat"}}
	other, _ := core.NewEnvironmentInstanceID()
	if _, err := service.ExecRunStream(ctx, env.Name, other, request, strings.NewReader(""), io.Discard, io.Discard); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("wrong creation executed", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := service.ExecRunStream(ctx, env.Name, id, request, strings.NewReader(""), io.Discard, io.Discard)
		done <- err
	}()
	<-runtime.started
	deleteCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := service.DeleteRun(deleteCtx, env.Name, id); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deletion raced executing process", err)
	}
	if len(runtime.deleteRefs) != 0 {
		t.Fatal("live process lost runtime")
	}
	close(runtime.finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteRun(ctx, env.Name, id); err != nil {
		t.Fatal(err)
	}
}
