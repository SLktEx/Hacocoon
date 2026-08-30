package incus

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRealIncusNerdctlUsesHostOCIWorkloadE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS") != "1" {
		t.Skip("set HACO_E2E_INCUS=1 on a supported Incus host")
	}
	if goruntime.GOOS != "linux" {
		t.Skip("real Incus E2E requires Linux/WSL2")
	}
	if _, err := exec.LookPath("incus"); err != nil {
		t.Fatalf("incus CLI not found: %v", err)
	}
	if strings.TrimSpace(os.Getenv(workloadShimBinaryEnvironment)) == "" {
		t.Fatal("HACO_WORKLOAD_SHIM_BINARY must point to the packaged haco-host companion for native workload E2E")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := e2eLoggingRunner{t: t, inner: host.ExecRunner{}}
	project := "haco-e2e-native-" + time.Now().Format("150405.000000000")
	project = strings.ReplaceAll(project, ".", "-")
	runtimeAdapter := New(runner)
	runtimeAdapter.project = project
	if err := runtimeAdapter.Prepare(ctx, core.RuntimePrepareSpec{StorageAttachment: map[string]string{"incus_pool": "default"}}); err != nil {
		t.Fatalf("prepare Incus runtime: %v", err)
	}

	environment := "native"
	environmentRef := "haco-" + environment
	workloadName := "smoke"
	workloadRef, err := workloadRef(environment, workloadName)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx := context.Background()
		_, _ = runner.Run(cleanupCtx, "incus", "delete", workloadRef, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "delete", environmentRef, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "project", "delete", project)
	}()

	brokerPath, err := WorkloadBrokerSocketPath(environment)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := control.ListenUnix(brokerPath, 0o600)
	if err != nil {
		t.Fatalf("listen on Environment workload broker: %v", err)
	}
	brokerServer := control.NewServer()
	if err := controlapi.RegisterBoundEnvironmentWorkloads(brokerServer, runtimeAdapter, environment); err != nil {
		_ = listener.Close()
		t.Fatalf("register Environment workload broker: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- brokerServer.Serve(ctx, listener) }()
	defer func() {
		cancel()
		_ = listener.Close()
		select {
		case serveErr := <-serveDone:
			if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
				t.Errorf("workload broker serve: %v", serveErr)
			}
		case <-time.After(5 * time.Second):
			t.Error("workload broker did not stop")
		}
	}()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseProvider, err := NewSandboxProvider(runtimeAdapter)
	if err != nil {
		t.Fatal(err)
	}
	nativeProvider, err := NewNativeWorkloadProvider(baseProvider)
	if err != nil {
		t.Fatal(err)
	}
	created, err := nativeProvider.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{
		Name:          environment,
		WorkspacePath: workspaceDir,
	})
	if err != nil {
		t.Fatalf("create native Environment: %v", err)
	}
	if created.Ref != environmentRef {
		t.Fatalf("Environment ref = %q, want %q", created.Ref, environmentRef)
	}

	version, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{"nerdctl", "--version"}})
	if err != nil || version.ExitCode != 0 || !strings.Contains(version.Stdout, "Hacocoon Incus OCI compatibility shim") {
		t.Fatalf("Environment nerdctl shim result=%#v err=%v", version, err)
	}

	run, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{
		"nerdctl", "run", "--name", workloadName, "busybox:1.37", "sleep", "120",
	}})
	if err != nil || run.ExitCode != 0 {
		t.Fatalf("Environment nerdctl run result=%#v err=%v", run, err)
	}
	if !strings.Contains(run.Stdout, workloadRef) {
		t.Fatalf("nerdctl run stdout %q does not identify %s", run.Stdout, workloadRef)
	}

	kind, err := runner.Run(ctx, "incus", "config", "get", workloadRef, workloadKindKey, "--project", project)
	if err != nil {
		t.Fatalf("inspect sibling OCI workload: %v", err)
	}
	if strings.TrimSpace(kind.Stdout) != workloadKindValue {
		t.Fatalf("workload marker = %q, want %q", strings.TrimSpace(kind.Stdout), workloadKindValue)
	}

	ps, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{"nerdctl", "ps"}})
	if err != nil || ps.ExitCode != 0 || !strings.Contains(ps.Stdout, workloadName) || !strings.Contains(ps.Stdout, "library/busybox:1.37") {
		t.Fatalf("Environment nerdctl ps result=%#v err=%v", ps, err)
	}

	execResult, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{
		"nerdctl", "exec", workloadName, "sh", "-c", "printf native-ok",
	}})
	if err != nil || execResult.ExitCode != 0 || execResult.Stdout != "native-ok" {
		t.Fatalf("Environment nerdctl exec result=%#v err=%v", execResult, err)
	}

	stop, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{"nerdctl", "stop", workloadName}})
	if err != nil || stop.ExitCode != 0 {
		t.Fatalf("Environment nerdctl stop result=%#v err=%v", stop, err)
	}
	rm, err := runtimeAdapter.ExecEnvironment(ctx, environmentRef, core.ExecutionRequest{Argv: []string{"nerdctl", "rm", workloadName}})
	if err != nil || rm.ExitCode != 0 {
		t.Fatalf("Environment nerdctl rm result=%#v err=%v", rm, err)
	}
	if _, err := runner.Run(ctx, "incus", "info", workloadRef, "--project", project); err == nil {
		t.Fatalf("sibling OCI workload %s still exists after nerdctl rm", workloadRef)
	}

	if err := nativeProvider.DeleteEnvironment(ctx, environmentRef); err != nil {
		t.Fatalf("delete native Environment: %v", err)
	}
}
