package incus

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs"
)

const realE2EEnv = "HACO_E2E_INCUS"

func TestRealIncusLifecycleE2E(t *testing.T) {
	if os.Getenv(realE2EEnv) != "1" {
		t.Skipf("set %s=1 to run the real Incus E2E test", realE2EEnv)
	}
	if runtime.GOOS != "linux" {
		t.Skip("real Incus E2E requires Linux")
	}
	if os.Geteuid() != 0 {
		t.Skip("real Incus E2E requires root for loop devices and Btrfs mounts")
	}

	requireHostCommands(t,
		"incus",
		"btrfs",
		"mkfs.btrfs",
		"blkid",
		"findmnt",
		"mount",
		"umount",
		"losetup",
		"truncate",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	runner := host.ExecRunner{}
	if result, err := runner.Run(ctx, "incus", "info"); err != nil {
		t.Fatalf("Incus daemon is not ready: %v\nstdout:\n%s\nstderr:\n%s", err, result.Stdout, result.Stderr)
	}

	suffix := fmt.Sprintf("%d-%x", os.Getpid(), time.Now().UnixNano()&0xfffffff)
	project := "hacocoon-e2e-" + suffix
	storageID := "e2e-" + suffix
	sessionID := core.SessionID("e2e-" + suffix)

	storageRoot := filepath.Join(t.TempDir(), "storage")
	storage, err := btrfs.NewLocal(ctx, storageRoot, runner, "raw")
	if err != nil {
		t.Fatalf("compose real Btrfs storage: %v", err)
	}
	caps, err := storage.Probe(ctx)
	if err != nil {
		t.Fatalf("probe real Btrfs storage: %v", err)
	}
	if !caps.Available {
		t.Fatalf("real Btrfs storage unavailable: %v", caps.Details)
	}

	const initialBytes int64 = 8 << 30
	handle, err := storage.Ensure(ctx, core.StorageSpec{ID: storageID, SizeBytes: initialBytes})
	if err != nil {
		t.Fatalf("ensure real Btrfs storage: %v", err)
	}

	rt := New(runner)
	rt.project = project

	var sessionRef string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()

		if sessionRef != "" {
			_ = rt.Delete(cleanupCtx, sessionRef)
		}
		_, _ = runner.Run(cleanupCtx, "incus", "delete", imageBuilderName, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "image", "delete", preparedImageAlias, "--project", project)
		if pool := handle.Attachment["incus_pool"]; pool != "" {
			_, _ = runner.Run(cleanupCtx, "incus", "storage", "delete", pool, "--project", project)
		}
		_, _ = runner.Run(cleanupCtx, "incus", "project", "delete", project)
		_ = storage.Delete(cleanupCtx, handle)
	})

	t.Run("prepare runtime and base image", func(t *testing.T) {
		if err := rt.Prepare(ctx, core.RuntimePrepareSpec{StorageAttachment: cloneStringMap(handle.Attachment)}); err != nil {
			t.Fatalf("prepare runtime: %v", err)
		}

		result, err := runner.Run(ctx, "incus", "image", "show", preparedImageAlias, "--project", project)
		if err != nil {
			t.Fatalf("prepared image alias missing: %v\nstdout:\n%s\nstderr:\n%s", err, result.Stdout, result.Stderr)
		}
	})

	t.Run("create and verify nested workload", func(t *testing.T) {
		created, err := rt.Create(ctx, core.RuntimeSessionSpec{
			ID:                sessionID,
			Name:              "real-e2e",
			StorageAttachment: cloneStringMap(handle.Attachment),
		})
		if err != nil {
			t.Fatalf("create real Session: %v", err)
		}
		sessionRef = created.Ref

		state, err := rt.Inspect(ctx, sessionRef)
		if err != nil {
			t.Fatalf("inspect running Session: %v", err)
		}
		if state.Observed != core.ObservedRunning {
			t.Fatalf("expected running Session, got %s", state.Observed)
		}

		if got := execInSession(t, ctx, rt, sessionRef, "cat", "/proc/1/comm"); got != "systemd" {
			t.Fatalf("PID 1 = %q, want systemd", got)
		}
		execInSession(t, ctx, rt, sessionRef, "systemctl", "is-active", "--quiet", "containerd")
		execInSession(t, ctx, rt, sessionRef, "nerdctl", "info")
		if got := execInSession(t, ctx, rt, sessionRef, "nerdctl", "run", "--rm", nestedSmokeImage, "echo", "hacocoon-e2e"); got != "hacocoon-e2e" {
			t.Fatalf("nested container output = %q, want hacocoon-e2e", got)
		}
	})

	t.Run("stop grow plan-shrink and restart", func(t *testing.T) {
		if err := rt.Stop(ctx, sessionRef); err != nil {
			t.Fatalf("stop Session: %v", err)
		}
		state, err := rt.Inspect(ctx, sessionRef)
		if err != nil {
			t.Fatalf("inspect stopped Session: %v", err)
		}
		if state.Observed != core.ObservedStopped {
			t.Fatalf("expected stopped Session, got %s", state.Observed)
		}

		before, err := storage.Inspect(ctx, handle)
		if err != nil {
			t.Fatalf("inspect storage before grow: %v", err)
		}
		growTarget := before.LogicalBytes + (1 << 30)
		if err := storage.Grow(ctx, handle, growTarget); err != nil {
			t.Fatalf("grow storage to %d: %v", growTarget, err)
		}
		after, err := storage.Inspect(ctx, handle)
		if err != nil {
			t.Fatalf("inspect storage after grow: %v", err)
		}
		if after.LogicalBytes < growTarget {
			t.Fatalf("storage logical size = %d, want at least %d", after.LogicalBytes, growTarget)
		}

		plan, err := storage.PlanShrink(ctx, handle, before.LogicalBytes)
		if err != nil {
			t.Fatalf("plan non-destructive shrink: %v", err)
		}
		if plan.TargetBytes != before.LogicalBytes {
			t.Fatalf("shrink plan target = %d, want %d", plan.TargetBytes, before.LogicalBytes)
		}

		if err := rt.Start(ctx, sessionRef); err != nil {
			t.Fatalf("restart Session: %v", err)
		}
		if got := execInSession(t, ctx, rt, sessionRef, "systemctl", "is-active", "containerd"); got != "active" {
			t.Fatalf("containerd after restart = %q, want active", got)
		}
	})
}

func requireHostCommands(t *testing.T, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			t.Fatalf("required host command %q not found in PATH", command)
		}
	}
}

func execInSession(t *testing.T, ctx context.Context, rt *Runtime, ref string, argv ...string) string {
	t.Helper()
	result, err := rt.Exec(ctx, ref, core.ExecRequest{Argv: argv})
	if err != nil {
		t.Fatalf("session exec %q failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(argv, " "), err, result.Stdout, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("session exec %q exited %d\nstdout:\n%s\nstderr:\n%s", strings.Join(argv, " "), result.ExitCode, result.Stdout, result.Stderr)
	}
	return strings.TrimSpace(result.Stdout)
}

func cloneStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
