package incus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
)

func TestRealIncusWorkspaceLifecycleE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS") != "1" {
		t.Skip("set HACO_E2E_INCUS=1 on a supported Incus host")
	}
	if goruntime.GOOS != "linux" {
		t.Skip("real Incus E2E requires Linux/WSL2")
	}
	if _, err := exec.LookPath("incus"); err != nil {
		t.Fatalf("incus CLI not found: %v", err)
	}

	ctx := context.Background()
	runner := host.ExecRunner{}
	if result, err := runner.Run(ctx, "incus", "version"); err != nil {
		t.Fatalf("Incus daemon is not usable: %v\n%s", err, result.Stderr)
	}

	project := fmt.Sprintf("haco-e2e-%d", time.Now().UnixNano())
	runtimeAdapter := New(runner)
	runtimeAdapter.project = project
	var shellStdout bytes.Buffer
	var shellStderr bytes.Buffer
	runtimeAdapter.stdin = strings.NewReader("exit\n")
	runtimeAdapter.stdout = &shellStdout
	runtimeAdapter.stderr = &shellStderr

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "host.txt"), []byte("from-host\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	service := workspaceapp.New(runtimeAdapter, store)
	instanceRef := "haco-demo"
	readonlyRef := "haco-readonly"
	defer func() {
		cleanupCtx := context.Background()
		_, _ = runner.Run(cleanupCtx, "incus", "delete", instanceRef, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "delete", readonlyRef, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "project", "delete", project)
	}()

	environment, err := service.Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: workspaceDir})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if environment.RuntimeRef != instanceRef {
		t.Fatalf("runtime ref = %q", environment.RuntimeRef)
	}

	readResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"cat", "/workspace/host.txt"}})
	if err != nil || readResult.ExitCode != 0 || readResult.Stdout != "from-host\n" {
		t.Fatalf("host -> environment read result=%#v err=%v", readResult, err)
	}

	writeResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"sh", "-c", "printf 'from-environment\\n' > /workspace/environment.txt"}})
	if err != nil || writeResult.ExitCode != 0 {
		t.Fatalf("environment write result=%#v err=%v", writeResult, err)
	}
	hostVisible, err := os.ReadFile(filepath.Join(workspaceDir, "environment.txt"))
	if err != nil {
		t.Fatalf("read host-visible environment file: %v", err)
	}
	if string(hostVisible) != "from-environment\n" {
		t.Fatalf("host-visible file = %q", hostVisible)
	}

	exitResult, err := service.Exec(ctx, "demo", core.ExecutionRequest{Argv: []string{"sh", "-c", "printf 'stdout-ok'; printf 'stderr-ok' >&2; exit 17"}})
	if err == nil || exitResult.ExitCode != 17 || exitResult.Stdout != "stdout-ok" || exitResult.Stderr != "stderr-ok" {
		t.Fatalf("exit propagation result=%#v err=%v", exitResult, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("expected exit code 17, got %T %v", err, err)
	}

	if err := service.Shell(ctx, "demo"); err != nil {
		t.Fatalf("shell connectivity: %v\nstderr=%s", err, shellStderr.String())
	}

	config, err := runner.Run(ctx, "incus", "config", "show", instanceRef, "--expanded", "--project", project)
	if err != nil {
		t.Fatalf("inspect Incus environment config: %v", err)
	}
	if !strings.Contains(config.Stdout, workspaceDir) {
		t.Fatalf("requested workspace is not present in expanded config:\n%s", config.Stdout)
	}
	for _, forbidden := range credentialExposureMarkers(t) {
		if forbidden != "" && strings.Contains(config.Stdout, forbidden) {
			t.Fatalf("unexpected credential/authority exposure marker %q in Incus config:\n%s", forbidden, config.Stdout)
		}
	}

	if err := service.Delete(ctx, "demo"); err != nil {
		t.Fatalf("delete environment: %v", err)
	}
	if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("metadata remains after delete: %v", err)
	}
	if _, err := runner.Run(ctx, "incus", "info", instanceRef, "--project", project); err == nil {
		t.Fatalf("Incus environment %s still exists after delete", instanceRef)
	}

	readonlyEnvironment, err := service.Create(ctx, core.EnvironmentSpec{Name: "readonly", WorkspacePath: workspaceDir, AccessMode: core.WorkspaceReadOnly})
	if err != nil {
		t.Fatalf("create read-only environment: %v", err)
	}
	if readonlyEnvironment.RuntimeRef != readonlyRef {
		t.Fatalf("read-only runtime ref = %q", readonlyEnvironment.RuntimeRef)
	}
	writeReadonly, err := service.Exec(ctx, "readonly", core.ExecutionRequest{Argv: []string{"sh", "-c", "touch /workspace/should-fail"}})
	if err == nil || writeReadonly.ExitCode == 0 {
		t.Fatalf("read-only workspace unexpectedly allowed write: result=%#v err=%v", writeReadonly, err)
	}
	if err := service.Delete(ctx, "readonly"); err != nil {
		t.Fatalf("delete read-only environment: %v", err)
	}
}

func credentialExposureMarkers(t *testing.T) []string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	markers := []string{
		"/var/lib/incus/unix.socket",
		"/var/lib/incus/unix.socket.user",
	}
	if home != "" {
		markers = append(markers, home+"/.ssh", home+"/.aws", home+"/.config/gh")
	}
	return markers
}
