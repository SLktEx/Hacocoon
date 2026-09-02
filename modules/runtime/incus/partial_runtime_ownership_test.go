package incus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSandboxProviderReturnsKnownRefWhenCleanupIsAmbiguous(t *testing.T) {
	runner := &fakeRunner{run: func(ctx context.Context, _ int, _ string, args []string) (host.Result, error) {
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
		if len(args) > 0 && args[0] == "delete" {
			<-ctx.Done()
			return host.Result{}, ctx.Err()
		}
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	provider.cleanupTimeout = 20 * time.Millisecond

	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/tmp/workspace",
		Resources: core.ResourceBudget{
			CPU: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 2},
		},
	})
	if !errors.Is(err, core.ErrIncompatibleState) || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if created.Ref != "haco-demo" {
		t.Fatalf("runtime ref = %q, want exact ambiguous Incus instance ref", created.Ref)
	}
}
