package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureSeedEnvironmentRuntimeReadyRetriesOnlyTransientSystemdStartup(t *testing.T) {
	startCalls := 0
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "systemctl start containerd.service hacocoon-docker.socket") {
			startCalls++
			if startCalls == 1 {
				return host.Result{
					ExitCode: 1,
					Stderr:   "Failed to connect to system scope bus via local transport: No such file or directory",
				}, errors.New("exit status 1")
			}
			return host.Result{}, nil
		}
		if strings.Contains(joined, "systemctl show -p ActiveState --value containerd.service") {
			return host.Result{Stdout: "active\n"}, nil
		}
		if strings.Contains(joined, "systemctl show -p ActiveState --value hacocoon-docker.socket") {
			return host.Result{Stdout: "active\n"}, nil
		}
		if strings.Contains(joined, "systemctl show -p ActiveState --value hacocoon-docker.service") {
			return host.Result{Stdout: "inactive\n"}, nil
		}
		return host.Result{}, errors.New("unexpected call: " + joined)
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	if err := provider.ensureSeedEnvironmentRuntimeReady(context.Background(), "haco-seed-env"); err != nil {
		t.Fatal(err)
	}
	if startCalls != 2 {
		t.Fatalf("startCalls=%d want=2", startCalls)
	}
}

func TestEnsureSeedEnvironmentRuntimeReadyFailsClosedOnUnitError(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "systemctl start containerd.service hacocoon-docker.socket") {
			return host.Result{ExitCode: 5, Stderr: "Failed to start containerd.service: Unit containerd.service not found."}, errors.New("exit status 5")
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	err = provider.ensureSeedEnvironmentRuntimeReady(context.Background(), "haco-seed-env")
	if err == nil || !strings.Contains(err.Error(), "Unit containerd.service not found") {
		t.Fatalf("err=%v", err)
	}
}
