//go:build linux

package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestOwnershipLockReleasedAfterSIGKILLAllowsRecovery(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "locks")
	const environmentID = "run-crashed-owner"

	cmd := exec.Command(os.Args[0], "-test.run=TestEphemeralOwnershipLockHelper")
	cmd.Env = append(os.Environ(),
		"HACO_TEST_EPHEMERAL_LOCK_HELPER=1",
		"HACO_TEST_EPHEMERAL_LOCK_DIR="+lockDir,
		"HACO_TEST_EPHEMERAL_ENVIRONMENT="+environmentID,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("helper did not acquire lock: text=%q err=%v", scanner.Text(), scanner.Err())
	}

	run := core.EphemeralRun{EnvironmentID: environmentID, State: core.EphemeralRunActive, CreatedAt: time.Now().UTC()}
	store := newFakeRunStore(run)
	env := &fakeEnvironments{}
	service := NewWithRecovery(env, store, lockDir)

	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(env.calls) != 0 {
		t.Fatalf("live helper-owned run was deleted: %v", env.calls)
	}
	if _, ok := store.runs[environmentID]; !ok {
		t.Fatal("live helper-owned marker was removed")
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("SIGKILL helper unexpectedly exited successfully")
	}
	waited = true

	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(env.calls) != 1 || env.calls[0] != "delete:"+environmentID {
		t.Fatalf("crashed run was not recovered exactly once: %v", env.calls)
	}
	if _, ok := store.runs[environmentID]; ok {
		t.Fatal("crashed run marker remained after recovery")
	}
}

func TestEphemeralOwnershipLockHelper(t *testing.T) {
	if os.Getenv("HACO_TEST_EPHEMERAL_LOCK_HELPER") != "1" {
		return
	}
	lockDir := os.Getenv("HACO_TEST_EPHEMERAL_LOCK_DIR")
	environmentID := os.Getenv("HACO_TEST_EPHEMERAL_ENVIRONMENT")
	lock, acquired, err := acquireOwnershipLock(lockDir, environmentID, true)
	if err != nil || !acquired {
		fmt.Fprintf(os.Stderr, "acquire helper lock: acquired=%t err=%v\n", acquired, err)
		os.Exit(2)
	}
	defer lock.Release()
	fmt.Println("ready")
	select {}
}

func TestSIGTERMDuringServiceRunPerformsBoundedCleanup(t *testing.T) {
	cleanupPath := filepath.Join(t.TempDir(), "cleanup-proof")
	cmd := exec.Command(os.Args[0], "-test.run=TestSIGTERMServiceRunHelper")
	cmd.Env = append(os.Environ(),
		"HACO_TEST_RUN_SIGNAL_HELPER=1",
		"HACO_TEST_RUN_SIGNAL_CLEANUP="+cleanupPath,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("service helper not ready: text=%q err=%v", scanner.Text(), scanner.Err())
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("SIGTERM run helper failed: %v", err)
	}
	waited = true
	if contents, err := os.ReadFile(cleanupPath); err != nil || string(contents) != "cleaned" {
		t.Fatalf("cleanup proof=%q err=%v", contents, err)
	}
}

func TestSIGTERMServiceRunHelper(t *testing.T) {
	if os.Getenv("HACO_TEST_RUN_SIGNAL_HELPER") != "1" {
		return
	}
	env := &signalCleanupEnvironment{cleanupPath: os.Getenv("HACO_TEST_RUN_SIGNAL_CLEANUP")}
	service := New(env)
	service.newName = func() (string, error) { return "run-signal-helper", nil }
	service.cleanupTimeout = time.Second
	result, err := service.Run(context.Background(), Spec{WorkspacePath: "/work/helper", Argv: []string{"block"}})
	if !errors.Is(err, context.Canceled) || !result.CleanedUp {
		fmt.Fprintf(os.Stderr, "result=%#v err=%v\n", result, err)
		os.Exit(3)
	}
	os.Exit(0)
}

type signalCleanupEnvironment struct {
	cleanupPath string
}

func (*signalCleanupEnvironment) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return core.Environment{Name: spec.Name}, nil
}

func (*signalCleanupEnvironment) Exec(ctx context.Context, _ string, _ core.ExecutionRequest) (core.ExecutionResult, error) {
	fmt.Println("ready")
	<-ctx.Done()
	return core.ExecutionResult{}, ctx.Err()
}

func (e *signalCleanupEnvironment) Delete(ctx context.Context, _ string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("cleanup inherited signal cancellation: %w", ctx.Err())
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("cleanup context has no deadline")
	}
	return os.WriteFile(e.cleanupPath, []byte("cleaned"), 0o600)
}
