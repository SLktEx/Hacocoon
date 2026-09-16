package clientforward

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// Use a private session so a foreground-group interrupt cannot reach the test
// runner, the user's terminal, or another process. An interop relay must survive
// that signal long enough for the owning parent's pipe cancellation to finish.
func TestCompanionForegroundInterrupt(t *testing.T) {
	if os.Getenv("HACO_TEST_TUNNEL_FOREGROUND") == "1" {
		if syscall.Getpgrp() != os.Getpid() {
			t.Fatal("test does not own its process group")
		}
		testRealCompanionCancellation(t, windowsCompanionCommand, func(cancel context.CancelFunc) {
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, syscall.SIGINT)
			defer signal.Stop(signals)
			if err := syscall.Kill(-syscall.Getpgrp(), syscall.SIGINT); err != nil {
				t.Fatal(err)
			}
			select {
			case <-signals:
				cancel()
			case <-time.After(time.Second):
				t.Fatal("foreground interrupt not delivered")
			}
		})
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCompanionForegroundInterrupt$", "-test.v")
	cmd.Env = append(os.Environ(), "HACO_TEST_TUNNEL_FOREGROUND=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("owned foreground interruption: %v\n%s", err, output)
	}
}
