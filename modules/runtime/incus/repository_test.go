package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func TestVolumeCopySetsExactOwnerInCreationRequest(t *testing.T) {
	source := gitrepo.Object{Kind: "repo", ID: "demo", Repository: "demo", NativeRef: "haco-local-default/haco-repo-demo", Owner: strings.Repeat("a", 32)}
	target := gitrepo.Object{Kind: "work", ID: "work", Repository: "demo", NativeRef: "haco-local-default/haco-work-work", Owner: strings.Repeat("b", 32)}
	posts := 0
	idmap := `[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":1000000000},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":1000000000}]`
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) == 2 && args[0] == "query" {
			config := volumeConfig(source)
			config["volatile.idmap.last"] = idmap
			config["volatile.idmap.next"] = idmap
			data, _ := json.Marshal(map[string]any{"name": "haco-repo-demo", "type": "custom", "content_type": "filesystem", "config": config})
			return host.Result{Stdout: string(data)}, nil
		}
		if len(args) != 7 || args[0] != "query" || args[2] != "POST" || args[3] != "--wait" {
			t.Fatalf("unexpected provider mutation: %v", args)
		}
		var request struct {
			Name   string            `json:"name"`
			Config map[string]string `json:"config"`
			Source map[string]any    `json:"source"`
		}
		if json.Unmarshal([]byte(args[6]), &request) != nil {
			t.Fatalf("request args=%v", args)
		}
		if request.Name != "haco-work-work" || request.Config["user.hacocoon.owner"] != target.Owner || request.Source["name"] != "haco-repo-demo" || request.Source["type"] != "copy" {
			t.Fatalf("request=%+v", request)
		}
		if request.Config["volatile.idmap.last"] != idmap || request.Config["volatile.idmap.next"] != idmap {
			t.Fatal("copy lost Incus ID bookkeeping and would shift file owners twice")
		}
		posts++
		return host.Result{}, nil
	}}
	backend := &RepositoryBackend{Runtime: New(runner)}
	if err := backend.CreateVolume(context.Background(), target, &source); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("posts=%d", posts)
	}
}
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
