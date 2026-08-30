package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestLifecycleRejectsInvalidEnvironmentNamesBeforeStateOrRuntimeAccess(t *testing.T) {
	invalidNames := []string{
		"../demo",
		"demo/../../other",
		"--all",
		"Bad_Name",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			storeAccessErr := errors.New("store must not be accessed")

			t.Run("exec", func(t *testing.T) {
				runtime := &fakeEnvironmentRuntime{}
				store := newFakeEnvironmentStore()
				store.getErr = storeAccessErr

				_, err := New(runtime, store).Exec(context.Background(), name, core.ExecutionRequest{Argv: []string{"true"}})
				if !errors.Is(err, core.ErrInvalidArgument) || errors.Is(err, storeAccessErr) {
					t.Fatalf("error = %v", err)
				}
				if runtime.execRef != "" || runtime.execRequest.Argv != nil {
					t.Fatalf("runtime was reached: ref=%q request=%#v", runtime.execRef, runtime.execRequest)
				}
			})

			t.Run("shell", func(t *testing.T) {
				runtime := &fakeEnvironmentRuntime{}
				store := newFakeEnvironmentStore()
				store.getErr = storeAccessErr

				err := New(runtime, store).Shell(context.Background(), name)
				if !errors.Is(err, core.ErrInvalidArgument) || errors.Is(err, storeAccessErr) {
					t.Fatalf("error = %v", err)
				}
				if runtime.shellRef != "" {
					t.Fatalf("runtime was reached: ref=%q", runtime.shellRef)
				}
			})

			t.Run("delete", func(t *testing.T) {
				runtime := &fakeEnvironmentRuntime{}
				store := newFakeEnvironmentStore()
				store.getErr = storeAccessErr

				err := New(runtime, store).Delete(context.Background(), name)
				if !errors.Is(err, core.ErrInvalidArgument) || errors.Is(err, storeAccessErr) {
					t.Fatalf("error = %v", err)
				}
				if len(runtime.deleteRefs) != 0 || len(store.deleted) != 0 {
					t.Fatalf("state/runtime was reached: runtime=%#v store=%#v", runtime.deleteRefs, store.deleted)
				}
			})
		})
	}
}
