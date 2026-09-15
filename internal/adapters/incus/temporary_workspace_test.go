package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"reflect"
	"strings"
	"testing"
)

func TestTemporaryWorkspaceNeverBuildsHostDiskDevice(t *testing.T) {
	runner := &fakeRunner{}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.addWorkspaceDevice(context.Background(), "haco-temp", core.EnvironmentRuntimeSpec{TemporaryWorkspace: true, WorkspacePath: work.Path}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/root", "temporary:../../root", "temporary:-bad"} {
		if err := provider.addWorkspaceDevice(context.Background(), "haco-temp", core.EnvironmentRuntimeSpec{TemporaryWorkspace: true, WorkspacePath: path}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatal("temporary source reached Host mount preparation")
	}
}
func TestEnvironmentWorkingDirectoryPreservesLiteralArguments(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		want := []string{"exec", "haco-temp", "--project", "hacocoon", "--cwd", "/workspace", "--", "printf", "%s", "space ; $(literal)"}
		if name != "incus" || !reflect.DeepEqual(args, want) {
			t.Fatalf("args=%q", args)
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	if _, err := runtime.ExecEnvironment(context.Background(), "haco-temp", core.ExecutionRequest{WorkingDirectory: "/workspace", Argv: []string{"printf", "%s", "space ; $(literal)"}}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"--option", "relative", "/bad" + string(rune(0))} {
		if _, err := runtime.ExecEnvironment(context.Background(), "haco-temp", core.ExecutionRequest{WorkingDirectory: dir, Argv: []string{"true"}}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("dir=%q err=%v", dir, err)
		}
	}
	if len(runner.calls) != 1 {
		t.Fatal("invalid directory reached execution")
	}
	if strings.Contains(strings.Join(runner.calls[0].args, " "), "sh -c") {
		t.Fatal("rewritten into a shell command")
	}
}
