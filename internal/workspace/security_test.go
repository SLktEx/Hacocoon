package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestLifecycleRejectsTraversalEnvironmentNamesBeforeSideEffects(t *testing.T) {
	const malicious = "../../../etc/ssh/sshd_config"
	runtime := &fakeEnvironmentRuntime{}
	store := newFakeEnvironmentStore()
	service := New(runtime, store)

	if _, err := service.Exec(context.Background(), malicious, core.ExecutionRequest{Argv: []string{"true"}}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("exec err=%v", err)
	}
	if err := service.Shell(context.Background(), malicious); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("shell err=%v", err)
	}
	if err := service.Delete(context.Background(), malicious); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("delete err=%v", err)
	}

	if runtime.execRef != "" || runtime.shellRef != "" || len(runtime.deleteRefs) != 0 {
		t.Fatalf("runtime received invalid environment name: exec=%q shell=%q deletes=%v", runtime.execRef, runtime.shellRef, runtime.deleteRefs)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("store delete side effect occurred for invalid name: %v", store.deleted)
	}
}
