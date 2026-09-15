package workspace_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/adapters/incus"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
)

// This test provider owns the shell fixture's files as its native resources.
// Workspace supplies the real catalog and lifecycle transitions; Incus Runtime
// supplies the real child-process execution path. No network acceptance is
// implied by the fixture's creation commands.
type processFixtureProvider struct{ *incus.Runtime }

func (*processFixtureProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, core.ErrUnsupported
}

func (*processFixtureProvider) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	runner := host.ExecRunner{}
	created := core.EnvironmentRuntime{Ref: "haco-" + spec.Name}
	if _, err := runner.Run(ctx, "incus", "init", "fixture-image", created.Ref); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := record(created); err != nil {
		return created, err
	}
	if _, err := runner.Run(ctx, "incus", "config", "device", "add", created.Ref, "workspace", "disk", "source="+spec.WorkspacePath); err != nil {
		return created, err
	}
	_, err := runner.Run(ctx, "incus", "start", created.Ref)
	return created, err
}

func TestWorkspaceLifecycleCrossesRealProcessBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed fake Incus is for Linux/WSL test hosts")
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	fakeState := filepath.Join(root, "incus-state")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeState, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(binDir, "incus"), fakeIncusWorkspaceScript)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_INCUS_STATE", fakeState)

	workspaceDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "host.txt"), []byte("from-host\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runtimeAdapter := &processFixtureProvider{Runtime: incus.New(host.ExecRunner{})}
	store := state.NewEnvironmentJSONStore(filepath.Join(root, "haco", "environments.json"))
	service := workspaceapp.New(runtimeAdapter, store)
	ctx := context.Background()

	environment, err := service.Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: workspaceDir})
	if err != nil {
		t.Fatal(err)
	}
	if environment.RuntimeRef != "haco-demo" {
		t.Fatalf("runtime ref = %q", environment.RuntimeRef)
	}
	if _, err := os.Stat(filepath.Join(fakeState, "instance-haco-demo")); err != nil {
		t.Fatalf("fake Incus instance was not created: %v", err)
	}

	secondStore := state.NewEnvironmentJSONStore(filepath.Join(root, "haco", "environments.json"))
	secondService := workspaceapp.New(runtimeAdapter, secondStore)
	if _, err := secondService.Create(ctx, core.EnvironmentSpec{Name: "conflict", WorkspacePath: workspaceDir}); !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("second process-equivalent service did not observe rw lease conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fakeState, "instance-haco-conflict")); !os.IsNotExist(err) {
		t.Fatalf("conflicting environment reached Incus: %v", err)
	}

	readResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"cat", "/workspace/host.txt"}})
	if err != nil || readResult.ExitCode != 0 || readResult.Stdout != "from-host\n" {
		t.Fatalf("read result=%#v err=%v", readResult, err)
	}

	writeResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"sh", "-c", "printf 'from-environment\\n' > /workspace/environment.txt"}})
	if err != nil || writeResult.ExitCode != 0 {
		t.Fatalf("write result=%#v err=%v", writeResult, err)
	}
	contents, err := os.ReadFile(filepath.Join(workspaceDir, "environment.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "from-environment\n" {
		t.Fatalf("host-visible contents = %q", contents)
	}

	argsResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world"}})
	if err != nil || argsResult.Stdout != "hello world" {
		t.Fatalf("argument-boundary result=%#v err=%v", argsResult, err)
	}

	exitResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"sh", "-c", "printf 'boom\\n' >&2; exit 17"}})
	if err == nil || exitResult.ExitCode != 17 || exitResult.Stderr != "boom\n" {
		t.Fatalf("nonzero result=%#v err=%v", exitResult, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("exec error = %T %v", err, err)
	}

	if err := service.Shell(ctx, "demo"); err != nil {
		t.Fatalf("shell process path failed: %v", err)
	}

	if err := service.Delete(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fakeState, "instance-haco-demo")); !os.IsNotExist(err) {
		t.Fatalf("fake Incus instance remains: %v", err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("metadata remains after delete: %v", err)
	}

	if _, err := secondService.Create(ctx, core.EnvironmentSpec{Name: "after-delete", WorkspacePath: workspaceDir}); err != nil {
		t.Fatalf("workspace lease was not released after delete: %v", err)
	}
	if err := secondService.Delete(ctx, "after-delete"); err != nil {
		t.Fatalf("delete second environment: %v", err)
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

const fakeIncusWorkspaceScript = `#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
command_name="${1:-}"
[ "$#" -gt 0 ] && shift

case "$command_name" in
  init)
    image="${1:-}"; instance="${2:-}"
    [ -n "$image" ] && [ -n "$instance" ] || exit 2
    echo "STOPPED" > "$state/instance-$instance"
    ;;
  config)
    [ "${1:-}" = "device" ] || exit 2
    action="${2:-}"
    instance="${3:-}"
    case "$action" in
      add)
        shift 5
        source_path=""
        for arg in "$@"; do
          case "$arg" in
            source=*) source_path="${arg#source=}" ;;
          esac
        done
        [ -n "$source_path" ] || exit 2
        printf '%s\n' "$source_path" > "$state/workspace-$instance"
        ;;
      *) exit 2 ;;
    esac
    ;;
  start)
    instance="${1:-}"
    echo "RUNNING" > "$state/instance-$instance"
    ;;
  delete)
    instance="${1:-}"
    if [ ! -f "$state/instance-$instance" ]; then
      echo "Error: Instance not found" >&2
      exit 1
    fi
    rm -f "$state/instance-$instance" "$state/workspace-$instance"
    ;;
  exec)
    instance="${1:-}"
    shift
    while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do shift; done
    [ "$#" -gt 0 ] && shift
    [ "$#" -gt 0 ] || exit 2
    workspace="$(cat "$state/workspace-$instance")"
    executable="$1"
    shift
    case "$executable" in
      test)
        [ "${1:-}" = "-w" ] && [ "${2:-}" = "/workspace" ] || exit 2
        [ -w "$workspace" ]
        ;;
      cat)
        target="$(printf '%s' "${1:-}" | sed "s#^/workspace#$workspace#")"
        cat "$target"
        ;;
      printf)
        printf "$@"
        ;;
      sh)
        [ "${1:-}" = "-c" ] || exit 2
        script="${2:-}"
        translated="$(printf '%s' "$script" | sed "s#/workspace#$workspace#g")"
        sh -c "$translated"
        ;;
      /bin/bash)
        exit 0
        ;;
      *)
        "$executable" "$@"
        ;;
    esac
    ;;
  *)
    exit 2
    ;;
esac
`
