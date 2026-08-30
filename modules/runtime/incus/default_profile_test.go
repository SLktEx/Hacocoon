package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestPrepareEnsuresSandboxNetworkByDefault(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}

	if err := New(runner).Prepare(context.Background(), core.RuntimePrepareSpec{}); err != nil {
		t.Fatal(err)
	}

	seenProfile := false
	seenACL := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "profile show "+sandboxProfile) {
			seenProfile = true
		}
		if strings.Contains(joined, "network acl show "+sandboxEgressACL) {
			seenACL = true
		}
	}
	if !seenProfile || !seenACL {
		t.Fatalf("Prepare did not verify sandbox defaults: %#v", runner.calls)
	}
}

func TestPreparedEnvironmentUsesManagedRootPool(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	if err := runtime.Prepare(context.Background(), core.RuntimePrepareSpec{StorageAttachment: map[string]string{
		"incus_pool": "haco-test-pool",
		"driver":     "btrfs",
		"source":     "/var/lib/hacocoon/mounts/test",
	}}); err != nil {
		t.Fatal(err)
	}

	runner.calls = nil
	if _, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/tmp/workspace",
		ReadOnly:      true,
	}); err != nil {
		t.Fatal(err)
	}

	seenManagedPool := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "profile show default --project default") {
			t.Fatalf("prepared Environment fell back to Incus default profile: %#v", runner.calls)
		}
		if len(call.args) > 0 && call.args[0] == "init" && strings.Contains(joined, "--storage haco-test-pool") {
			seenManagedPool = true
		}
	}
	if !seenManagedPool {
		t.Fatalf("managed root pool missing from Environment init: %#v", runner.calls)
	}
}

func TestCreateSessionUsesSandboxProfileByDefault(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}

	created, err := New(runner).Create(context.Background(), core.RuntimeSessionSpec{ID: core.SessionID("abc123")})
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "haco-abc123" {
		t.Fatalf("ref = %q", created.Ref)
	}

	for _, call := range runner.calls {
		if len(call.args) > 0 && call.args[0] == "launch" {
			assertRunnerCall(t, call, "incus", "launch", defaultImage, "haco-abc123", "--project", defaultProject, "--profile", sandboxProfile)
			return
		}
	}
	t.Fatalf("launch call missing: %#v", runner.calls)
}
