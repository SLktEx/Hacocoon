package incus

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSeedHarvestRunnerUsesOnlyMarkedManagedEnvironment(t *testing.T) {
	ref := "private.example/app:stable@sha256:" + testFingerprintA
	directPull := false
	savedFrom := ""
	quarantineSeen := false
	seedLoadSeen := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case name == "incus" && len(args) >= 1 && args[0] == "list":
			return host.Result{Stdout: `[
				{"name":"haco-unmarked","status":"Running","config":{}},
				{"name":"haco-private","status":"Running","config":{"user.hacocoon.kind":"environment"}}
			]`}, nil
		case name == "incus" && strings.Contains(joined, "nerdctl save"):
			savedFrom = args[1]
			return host.Result{}, nil
		case name == "incus" && len(args) >= 4 && args[0] == "file" && args[1] == "pull":
			if err := os.WriteFile(args[3], []byte("environment-oci-archive"), 0o600); err != nil {
				return host.Result{}, err
			}
			return host.Result{}, nil
		case name == "incus" && strings.Contains(joined, " rm -f "):
			return host.Result{}, nil
		case name == "nerdctl" && len(args) == 3 && args[0] == "namespace" && args[1] == "create":
			if !strings.HasPrefix(args[2], seedHarvestNamespacePrefix) {
				t.Fatalf("unexpected quarantine namespace %q", args[2])
			}
			quarantineSeen = true
			return host.Result{}, nil
		case name == "nerdctl" && len(args) == 3 && args[0] == "namespace" && args[1] == "remove":
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 5 && args[0] == "--namespace" && args[2] == "load":
			if args[1] == seedHostNamespace {
				seedLoadSeen = true
			}
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 5 && args[0] == "--namespace" && args[2] == "image" && args[3] == "inspect":
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 6 && args[0] == "--namespace" && args[2] == "save":
			if err := os.WriteFile(args[4], []byte("selected-oci-archive"), 0o600); err != nil {
				return host.Result{}, err
			}
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 3 && args[2] == "pull":
			directPull = true
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected call: " + name + " " + joined)
		}
	}}

	wrapped := WrapSeedHarvestRunner(runner)
	if _, err := wrapped.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "pull", ref); err != nil {
		t.Fatal(err)
	}
	if directPull {
		t.Fatal("trusted Host registry pull should not run after successful Environment harvest")
	}
	if savedFrom != "haco-private" {
		t.Fatalf("saved from %q, want marked Environment", savedFrom)
	}
	if !quarantineSeen || !seedLoadSeen {
		t.Fatalf("quarantine=%t seed-load=%t calls=%#v", quarantineSeen, seedLoadSeen, runner.calls)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "haco-unmarked") && strings.Contains(joined, "nerdctl save") {
			t.Fatalf("unmarked Environment was inspected for harvest: %#v", call)
		}
		if strings.Contains(strings.ToLower(joined), "credential") || strings.Contains(joined, ".docker/config") {
			t.Fatalf("harvest command touched credential material: %#v", call)
		}
	}
}

func TestSeedHarvestRunnerFallsBackToHostPull(t *testing.T) {
	ref := "private.example/app:stable@sha256:" + testFingerprintA
	pullCalled := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name == "incus" && len(args) > 0 && args[0] == "list" {
			return host.Result{Stdout: `[{"name":"haco-old","status":"Running","config":{}}]`}, nil
		}
		if name == "nerdctl" && len(args) >= 3 && args[2] == "pull" {
			pullCalled = true
			return host.Result{}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	wrapped := WrapSeedHarvestRunner(runner)
	if _, err := wrapped.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "pull", ref); err != nil {
		t.Fatal(err)
	}
	if !pullCalled {
		t.Fatal("trusted Host pull fallback was not used")
	}
}

func TestSeedHarvestRunnerDoesNotInterceptMutablePull(t *testing.T) {
	called := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		called = true
		if name != "nerdctl" || args[len(args)-1] != "private.example/app:latest" {
			t.Fatalf("unexpected forwarded call: %s %#v", name, args)
		}
		return host.Result{}, nil
	}}
	wrapped := WrapSeedHarvestRunner(runner)
	if _, err := wrapped.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "pull", "private.example/app:latest"); err != nil {
		t.Fatal(err)
	}
	if !called || len(runner.calls) != 1 {
		t.Fatalf("mutable pull should be forwarded exactly once: %#v", runner.calls)
	}
}

func TestSeedHarvestCandidatesFailClosedOnMarkedInvalidSource(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{Stdout: `[{"name":"--bad","status":"Running","config":{"user.hacocoon.kind":"environment"}}]`}, nil
	}}
	harvest := &seedHarvestRunner{next: runner, project: defaultProject}
	if _, err := harvest.harvestCandidates(context.Background()); err == nil {
		t.Fatal("expected invalid marked source to fail closed")
	}
}

func TestSeedHarvestRunnerDoesNotHideQuarantineCleanupFailure(t *testing.T) {
	ref := "private.example/app:stable@sha256:" + testFingerprintA
	directPull := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case name == "incus" && len(args) > 0 && args[0] == "list":
			return host.Result{Stdout: `[{"name":"haco-private","status":"Running","config":{"user.hacocoon.kind":"environment"}}]`}, nil
		case name == "incus" && strings.Contains(joined, "nerdctl save"):
			return host.Result{}, nil
		case name == "incus" && len(args) >= 4 && args[0] == "file" && args[1] == "pull":
			if err := os.WriteFile(args[3], []byte("environment-oci-archive"), 0o600); err != nil {
				return host.Result{}, err
			}
			return host.Result{}, nil
		case name == "incus" && strings.Contains(joined, " rm -f "):
			return host.Result{}, nil
		case name == "nerdctl" && len(args) == 3 && args[0] == "namespace" && args[1] == "create":
			return host.Result{}, nil
		case name == "nerdctl" && len(args) == 3 && args[0] == "namespace" && args[1] == "remove":
			return host.Result{ExitCode: 1, Stderr: "busy"}, errors.New("namespace busy")
		case name == "nerdctl" && len(args) >= 5 && args[0] == "--namespace" && args[2] == "load":
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 5 && args[0] == "--namespace" && args[2] == "image" && args[3] == "inspect":
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 6 && args[0] == "--namespace" && args[2] == "save":
			if err := os.WriteFile(args[4], []byte("selected-oci-archive"), 0o600); err != nil {
				return host.Result{}, err
			}
			return host.Result{}, nil
		case name == "nerdctl" && len(args) >= 3 && args[2] == "pull":
			directPull = true
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected call")
		}
	}}
	wrapped := WrapSeedHarvestRunner(runner)
	_, err := wrapped.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "pull", ref)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v want ErrRecoveryRequired", err)
	}
	if directPull {
		t.Fatal("registry fallback must not hide ambiguous quarantine cleanup")
	}
}
