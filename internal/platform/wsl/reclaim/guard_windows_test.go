//go:build windows

package wslreclaim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestContinuationGuardCrossProcess(t *testing.T) {
	if value := os.Getenv("HACO_TEST_RECLAIM_GUARD"); value != "" {
		id, err := windows.GUIDFromString(value)
		if err != nil {
			t.Fatal(err)
		}
		guard, err := acquireContinuation(id)
		if os.Getenv("HACO_TEST_RECLAIM_EXPECT_BUSY") == "1" {
			if guard != nil || !errors.Is(err, errContinuationBusy) {
				t.Fatal(guard, err)
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if err := guard.Close(); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	guard, err := acquireContinuation(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := guard.Close(); err != nil {
			t.Error(err)
		}
	})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(busy string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, executable, "-test.run=^TestContinuationGuardCrossProcess$", "-test.v")
		child.Env = append(os.Environ(), "HACO_TEST_RECLAIM_GUARD="+id.String(), "HACO_TEST_RECLAIM_EXPECT_BUSY="+busy)
		if output, err := child.CombinedOutput(); err != nil {
			t.Fatalf("child: %v\n%s", err, output)
		}
	}
	run("1")
	// A failed contender must close its opened handle, or this remains busy.
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	run("0")
}

func TestContinuationGuardScopeAndRelease(t *testing.T) {
	if _, err := acquireContinuation(windows.GUID{}); err == nil {
		t.Fatal("zero GUID accepted")
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	first, err := acquireContinuation(id)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if guard, err := acquireContinuation(id); guard != nil || !errors.Is(err, errContinuationBusy) {
		t.Fatal(guard, err)
	}
	otherID, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	other, err := acquireContinuation(otherID)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal("close not idempotent", err)
	}
	next, err := acquireContinuation(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}
