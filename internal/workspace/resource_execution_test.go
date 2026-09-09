package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
	"time"
)

func TestResourceExecutionRejectsRecycledEnvironmentAndStore(t *testing.T) {
	for _, mode := range []string{"valid", "generation", "store-owner", "store-id"} {
		t.Run(mode, func(t *testing.T) {
			store, runtime := snapshotFixture()
			ref := core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("a", 32)}
			env := store.environments["resume"]
			env.PersistentResource = ref
			store.environments["resume"] = env
			instance := store.identity
			switch mode {
			case "generation":
				instance = "env-" + strings.Repeat("2", 32)
			case "store-owner":
				ref.Owner = strings.Repeat("b", 32)
			case "store-id":
				ref.ID = "oci:other"
			}
			_, err := New(runtime, store).ExecForResource(context.Background(), "resume", instance, ref, core.ExecutionRequest{Argv: []string{"true"}})
			if mode == "valid" {
				if err != nil || runtime.execRef != env.RuntimeRef {
					t.Fatalf("exec: %v %q", err, runtime.execRef)
				}
			} else if !errors.Is(err, core.ErrCapabilityStale) || runtime.execRef != "" {
				t.Fatalf("stale execution: %v %q", err, runtime.execRef)
			}
		})
	}
}

// The pending operation must retain its generation until the native call ends.
type heldResourceRuntime struct {
	fakeEnvironmentRuntime
	entered chan struct{}
	release chan struct{}
}

func (r *heldResourceRuntime) ExecEnvironment(ctx context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	close(r.entered)
	select {
	case <-ctx.Done():
		return core.ExecutionResult{}, ctx.Err()
	case <-r.release:
		return core.ExecutionResult{}, nil
	}
}
func TestResourceExecutionExcludesConcurrentEnvironmentDelete(t *testing.T) {
	store, _ := snapshotFixture()
	ref := core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("a", 32)}
	env := store.environments["resume"]
	env.PersistentResource = ref
	store.environments["resume"] = env
	runtime := &heldResourceRuntime{entered: make(chan struct{}), release: make(chan struct{})}
	service := New(runtime, store)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := service.ExecForResource(ctx, "resume", store.identity, ref, core.ExecutionRequest{Argv: []string{"true"}})
		done <- err
	}()
	select {
	case <-runtime.entered:
	case <-ctx.Done():
		t.Fatal("execution did not enter")
	}
	deletion, cancelDelete := context.WithTimeout(ctx, 50*time.Millisecond)
	err := service.Delete(deletion, "resume")
	cancelDelete()
	close(runtime.release)
	executionErr := <-done
	if !errors.Is(err, context.DeadlineExceeded) || executionErr != nil || len(runtime.deleteRefs) != 0 {
		t.Fatalf("concurrent delete escaped lock: delete=%v exec=%v native=%v", err, executionErr, runtime.deleteRefs)
	}
}
