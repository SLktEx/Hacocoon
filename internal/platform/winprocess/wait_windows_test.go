package winprocess

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Synthetic self-processes only: these fixtures never start WSL or open disks.
func TestPrivateRootWaitChild(t *testing.T) {
	switch os.Getenv("HACO_TEST_PRIVATE_WAIT") {
	case "success":
		os.Exit(0)
	case "failure":
		_, _ = io.WriteString(os.Stdout, "plausible result\n")
		os.Exit(7)
	case "blocked":
		time.Sleep(time.Minute)
		os.Exit(9)
	case "descendant":
		exe, err := os.Executable()
		if err != nil {
			os.Exit(10)
		}
		child := exec.Command(exe, "-test.run=^TestPrivateRootWaitChild$")
		child.Env = []string{"SystemRoot=" + os.Getenv("SystemRoot"), "HACO_TEST_PRIVATE_WAIT=blocked"}
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		if child.Start() != nil {
			os.Exit(11)
		}
		_ = child.Process.Release()
		os.Exit(0)
	}
}

func privateWaitFixture(t *testing.T, mode string) *Private {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		t.Fatal(err)
	}
	process, err := Start(exe, []string{"-test.run=^TestPrivateRootWaitChild$"},
		[]string{"SystemRoot=" + root, "HACO_TEST_PRIVATE_WAIT=" + mode}, filepath.Dir(exe))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Error(err)
		}
	})
	return process
}

func TestPrivateWaitRejectsNonzeroExitDespiteOutput(t *testing.T) {
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			process := privateWaitFixture(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := process.Wait(ctx)
			if (err == nil) != (mode == "success") {
				t.Fatal(err)
			}
			if err := process.Close(); err != nil {
				t.Fatal("root exit did not permit descendant cleanup", err)
			}
		})
	}
}

func TestPrivateWaitAndConcurrentCloseKeepExactHandleOwnership(t *testing.T) {
	process := privateWaitFixture(t, "blocked")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- process.Wait(ctx) }()
	var closers sync.WaitGroup
	for range 4 {
		closers.Go(func() {
			if err := process.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	closers.Wait()
	if err := <-result; err == nil {
		t.Fatal("terminated/closed process became a successful root exit")
	}
}

func TestPrivateRootExitStillRequiresDescendantCleanupAndLeavesPeers(t *testing.T) {
	peer := privateWaitFixture(t, "blocked")
	process := privateWaitFixture(t, "descendant")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := process.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	// The root exited but its Windows descendant retained the output handle.
	// Close still must use job membership, not root exit or pipe EOF.
	if err := process.Close(); err != nil {
		t.Fatal(err)
	}
	peerCtx, peerCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer peerCancel()
	if err := peer.Wait(peerCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unrelated privately owned peer was terminated", err)
	}
}

func TestPrivateCloseCachesRootHandleCloseFailure(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	read, write, err := os.Pipe()
	if err != nil {
		_ = windows.CloseHandle(job)
		t.Fatal(err)
	}
	// An empty job proves Windows descendant absence, but root-handle cleanup
	// failure must still remain visible in the cached result.
	process := &Private{Input: write, Output: read, job: job}
	for range 2 {
		if err := process.Close(); !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
			t.Fatal("root handle close failure was discarded", err)
		}
	}
}
