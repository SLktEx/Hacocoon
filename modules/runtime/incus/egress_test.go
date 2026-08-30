package incus

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestResolveRuntimeRefUsesIncusRuntimeState(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "ipv4=10.200.0.23" {
			return host.Result{Stdout: "haco-demo\n"}, nil
		}
		return host.Result{}, nil
	}}
	got, err := New(runner).ResolveRuntimeRef(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "haco-demo" {
		t.Fatalf("runtime ref = %q, want haco-demo", got)
	}
}

func TestResolveEnvironmentUsesIncusRuntimeState(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "ipv4=10.200.0.23" {
			return host.Result{Stdout: "haco-demo\n"}, nil
		}
		return host.Result{}, nil
	}}
	got, err := New(runner).ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "demo" {
		t.Fatalf("environment = %q, want demo", got)
	}
}

func TestResolveEnvironmentFailsClosedOnAmbiguousSource(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "list" {
			return host.Result{Stdout: "haco-a\nhaco-b\n"}, nil
		}
		return host.Result{}, nil
	}}
	_, err := New(runner).ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestResolveEnvironmentRejectsUnmanagedInstanceName(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "list" {
			return host.Result{Stdout: "other-instance\n"}, nil
		}
		return host.Result{}, nil
	}}
	_, err := New(runner).ResolveEnvironment(context.Background(), net.ParseIP("10.200.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestPrepareEgressProxyReturnsManagedBridgeAddress(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	address, err := New(runner).PrepareEgressProxy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if address != "10.200.0.1:18080" {
		t.Fatalf("address = %q", address)
	}
}
