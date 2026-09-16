package reviewcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	desktopreview "github.com/SLktEx/Hacocoon/internal/client/review"
)

func TestPrivateReviewDescendantFixture(t *testing.T) {
	mode := os.Getenv("HACO_TEST_REVIEW_DESCENDANT")
	if mode == "" {
		return
	}
	directory := os.Getenv("HACO_TEST_REVIEW_DIRECTORY")
	if mode == "wrapper" || mode == "orphan" {
		own, _ := os.Executable()
		cmd := exec.Command(own, "-test.run=^TestPrivateReviewDescendantFixture$")
		cmd.Env = append(os.Environ(), "HACO_TEST_REVIEW_DESCENDANT=child")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(3)
		}
		if mode == "wrapper" {
			_ = cmd.Wait()
		}
		os.Exit(0)
	}
	if mode != "child" {
		os.Exit(4)
	}
	if err := os.WriteFile(filepath.Join(directory, "ready"), []byte("ready"), 0600); err != nil {
		os.Exit(5)
	}
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		if _, err := os.Stat(filepath.Join(directory, "release")); err == nil {
			_ = os.WriteFile(filepath.Join(directory, "late"), []byte("started after peer closed"), 0600)
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(6)
}

func TestPrivateReviewCloseStopsDelayedDescendants(t *testing.T) {
	for _, mode := range []string{"wrapper", "orphan"} {
		t.Run(mode, func(t *testing.T) { testPrivateReviewDescendantCleanup(t, mode) })
	}
}

func testPrivateReviewDescendantCleanup(t *testing.T, mode string) {
	own, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	plan := desktopreview.Invocation{File: own, Args: []string{"-test.run=^TestPrivateReviewDescendantFixture$"}, Env: append(os.Environ(), "HACO_TEST_REVIEW_DESCENDANT="+mode, "HACO_TEST_REVIEW_DIRECTORY="+directory)}
	peer, err := startReviewPeer(context.Background(), plan, own)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	until := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("descendant did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	if err := peer.Ready(ctx); err == nil {
		cancel()
		t.Fatal("missing readiness accepted")
	}
	cancel()
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "release"), []byte("closed"), 0600); err != nil {
		t.Fatal(err)
	}
	until = time.Now().Add(time.Second)
	for time.Now().Before(until) {
		if _, err := os.Stat(filepath.Join(directory, "late")); err == nil {
			t.Fatal("private WSL launch descendant survived peer.Close and could restart after gate release")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPrivateReviewCleanupPreservesUnrelatedPeer(t *testing.T) {
	own, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	plan := desktopreview.Invocation{File: own, Args: []string{"-test.run=^TestNativePrivateReviewChild$"}, Env: []string{"HACO_REVIEW_TEST_CHILD=1", "SystemRoot=" + os.Getenv("SystemRoot")}}
	first, err := startReviewPeer(context.Background(), plan, own)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := startReviewPeer(context.Background(), plan, own)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := second.Ready(ctx); err != nil {
		t.Fatal("unrelated peer affected", err)
	}
}
