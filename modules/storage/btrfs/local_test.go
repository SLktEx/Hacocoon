package btrfs

import (
	"context"
	"fmt"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type localBackendRunner struct {
	calls []string
}

func (r *localBackendRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, fmt.Sprintf("%s %v", name, args))
	if name == "losetup" && len(args) == 1 && args[0] == "--version" {
		return host.Result{Stdout: "losetup test\n"}, nil
	}
	return host.Result{}, fmt.Errorf("unexpected command %s %v", name, args)
}

func TestChooseBlockBackendDefaultsToRaw(t *testing.T) {
	runner := &localBackendRunner{}
	backend, err := chooseBlockBackend(context.Background(), runner, "")
	if err != nil {
		t.Fatalf("choose default backend: %v", err)
	}
	if got, want := backend.ID(), "block.local-raw"; got != want {
		t.Fatalf("backend ID = %q, want %q", got, want)
	}
}

func TestChooseBlockBackendRejectsRemovedQcow2(t *testing.T) {
	runner := &localBackendRunner{}
	if _, err := chooseBlockBackend(context.Background(), runner, "qcow2"); err == nil {
		t.Fatal("expected removed qcow2 backend to be rejected")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("removed backend request unexpectedly probed host tools: %v", runner.calls)
	}
}
