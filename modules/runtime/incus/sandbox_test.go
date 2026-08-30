package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const sandboxTestFingerprint = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestSandboxProviderAppliesFiniteLimitsBeforeStart(t *testing.T) {
	values := map[string]string{}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"` + sandboxTestFingerprint + `"}`}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == "default" {
			return rootProfileResult(), nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "set" {
			parts := strings.SplitN(args[3], "=", 2)
			values[parts[0]] = parts[1]
			return host.Result{}, nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "get" {
			return host.Result{Stdout: values[args[3]] + "\n"}, nil
		}
		if len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "set" {
			parts := strings.SplitN(args[5], "=", 2)
			values["root."+parts[0]] = parts[1]
			return host.Result{}, nil
		}
		if len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "get" {
			return host.Result{Stdout: values["root."+args[5]] + "\n"}, nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	budget := core.ResourceBudget{
		CPU:         core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 4},
		MemoryBytes: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 8 << 30},
		PIDs:        core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 1024},
		RootBytes:   core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 40 << 30},
	}
	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/work", Resources: budget})
	if err != nil {
		t.Fatal(err)
	}
	if created.Resources != budget {
		t.Fatalf("resources = %#v, want %#v", created.Resources, budget)
	}
	start := -1
	resourceOps := 0
	seenManagedProfile := false
	for i, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if len(call.args) > 0 && call.args[0] == "start" {
			start = i
		}
		if strings.Contains(joined, "--profile "+sandboxProfile) {
			seenManagedProfile = true
		}
		if strings.Contains(joined, "limits.cpu=4") || strings.Contains(joined, "limits.memory=8589934592B") || strings.Contains(joined, "limits.processes=1024") || strings.Contains(joined, "size=42949672960B") {
			resourceOps++
			if start >= 0 {
				t.Fatalf("resource operation occurred after start: %#v", call)
			}
		}
	}
	if start < 0 {
		t.Fatal("start call missing")
	}
	if !seenManagedProfile {
		t.Fatalf("managed sandbox profile missing from init: %#v", runner.calls)
	}
	if resourceOps != 4 {
		t.Fatalf("resource set operations = %d, calls=%#v", resourceOps, runner.calls)
	}
}

func TestSandboxProviderVerificationFailureCleansUpWithoutStart(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"` + sandboxTestFingerprint + `"}`}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == "default" {
			return rootProfileResult(), nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "get" && args[3] == "limits.cpu" {
			return host.Result{Stdout: "999\n"}, nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name: "demo", WorkspacePath: "/tmp/work",
		Resources: core.ResourceBudget{CPU: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 2}},
	})
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	seenDelete := false
	for _, call := range runner.calls {
		if len(call.args) > 0 && call.args[0] == "start" {
			t.Fatalf("unexpected start after verification failure: %#v", call)
		}
		if len(call.args) > 0 && call.args[0] == "delete" {
			seenDelete = true
		}
	}
	if !seenDelete {
		t.Fatalf("cleanup delete missing: %#v", runner.calls)
	}
}
