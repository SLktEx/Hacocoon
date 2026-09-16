//go:build windows

package wslreclaim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/wslcoord"
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

func TestLaunchGateCrossProcess(t *testing.T) {
	if name := os.Getenv("HACO_TEST_LAUNCH_GUARD"); name != "" {
		guard, err := wslcoord.AcquireLaunch(name)
		if guard != nil || !errors.Is(err, wslcoord.ErrBusy) {
			t.Fatal("reclamation launch gate not shared", guard, err)
		}
		return
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	name := "Hacocoon-" + id.String()[1:37]
	guard, err := wslcoord.AcquireLaunch(name)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLaunchGateCrossProcess$")
	child.Env = append(os.Environ(), "HACO_TEST_LAUNCH_GUARD="+name)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	other, err := wslcoord.AcquireLaunch(name + "-other")
	if err != nil {
		t.Fatal("unrelated distribution blocked", err)
	}
	defer other.Close()
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := wslcoord.AcquireLaunch(name)
	if err != nil {
		t.Fatal("losing child leaked reservation", err)
	}
	defer next.Close()
	for _, invalid := range []string{"", "-bad", "a\\b", "a/b"} {
		if g, err := wslcoord.AcquireLaunch(invalid); g != nil || err == nil {
			t.Fatal(invalid, g, err)
		}
	}
}
