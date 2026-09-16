package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestManagedWorkspaceNeverFallsBackToHostPath(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	provider, err := NewSandboxProvider(runtime)
	if err != nil {
		t.Fatal(err)
	}
	spec := core.EnvironmentRuntimeSpec{WorkspacePath: "managed:work"}
	if err := provider.addWorkspaceDevice(context.Background(), "haco-dev", spec); !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatal("managed source fell through to host bind mount")
	}
	runtime.ConfigureManagedWorkspaces(func(context.Context, string) ([]WorkspaceAttachment, error) {
		return []WorkspaceAttachment{{Device: "workspace", Pool: "haco-local-default", Volume: "haco-work-work", Path: "/workspace"}}, nil
	})
	if err := provider.addWorkspaceDevice(context.Background(), "haco-dev", spec); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(runner.calls[0].args, " ")
	if !strings.Contains(args, "pool=haco-local-default source=haco-work-work path=/workspace") || strings.Contains(args, "shift=") || strings.Contains(args, "raw.idmap") {
		t.Fatalf("args=%s", args)
	}
}

func TestCollectionMountsAreIndependentAndReadOnlyIsPreserved(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	provider, err := NewSandboxProvider(runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ConfigureManagedWorkspaces(func(context.Context, string) ([]WorkspaceAttachment, error) {
		return []WorkspaceAttachment{
			{Device: "workspace-one", Pool: "haco-local-default", Volume: "haco-work-both-one", Path: "/workspace/one"},
			{Device: "workspace-two", Pool: "haco-local-default", Volume: "haco-work-both-two", Path: "/workspace/two"},
		}, nil
	})
	if err := provider.addWorkspaceDevice(context.Background(), "haco-dev", core.EnvironmentRuntimeSpec{WorkspacePath: "managed:both", ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%v", runner.calls)
	}
	for i, id := range []string{"one", "two"} {
		args := strings.Join(runner.calls[i].args, " ")
		if !strings.Contains(args, "source=haco-work-both-"+id+" path=/workspace/"+id) || !strings.Contains(args, "readonly=true") {
			t.Fatal(args)
		}
	}
	for _, mount := range []WorkspaceAttachment{
		{Device: "workspace-one", Pool: "haco-local-default", Volume: "haco-work-one", Path: "/mnt/c"},
		{Device: "workspace-../host", Pool: "haco-local-default", Volume: "haco-work-one", Path: "/workspace/../host"},
	} {
		if validWorkspaceAttachment(mount) {
			t.Fatalf("unsafe mount accepted: %+v", mount)
		}
	}
}
