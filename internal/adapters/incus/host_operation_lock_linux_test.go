//go:build linux

package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestHostOperationLockExcludesAnotherProcess(t *testing.T) {
	project := t.TempDir()
	unlock, err := lockHostOperation(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	child := func(mode string) {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestHostOperationLockProcessHelper$")
		command.Env = append(os.Environ(), "HACO_HOST_LOCK_TEST_PROJECT="+project, "HACO_HOST_LOCK_TEST_MODE="+mode)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("process lock: %v %s", err, output)
		}
	}
	child("busy")
	unlock()
	child("free")
}
func TestHostOperationLockProcessHelper(t *testing.T) {
	project := os.Getenv("HACO_HOST_LOCK_TEST_PROJECT")
	if project == "" {
		t.Skip("process helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	unlock, err := lockHostOperation(ctx, project)
	if os.Getenv("HACO_HOST_LOCK_TEST_MODE") == "busy" {
		if unlock != nil {
			unlock()
		}
		if !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal("another process bypassed Host lock")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
