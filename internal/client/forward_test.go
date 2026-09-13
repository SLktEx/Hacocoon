package client

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"testing"
)

type forwardCatalog struct {
	fakeStore
	instance string
}

func (s *forwardCatalog) GetReadyEnvironment(ctx context.Context, name string) (core.Environment, error) {
	return s.GetEnvironment(ctx, name)
}
func (s *forwardCatalog) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return s.instance, nil
}

type forwardRuntime struct {
	fakeRuntime
	calls                  int
	ref, instance, address string
	port                   int
}

func (r *forwardRuntime) DialEnvironmentTCP(_ context.Context, ref, instance, address string, port int) (net.Conn, error) {
	r.calls++
	r.ref = ref
	r.instance = instance
	r.address = address
	r.port = port
	return nil, core.ErrRuntimeUnavailable
}

func TestTCPForwardRequiresCurrentReadyCreationAndPinsProvider(t *testing.T) {
	ctx := context.Background()
	id, _ := core.NewEnvironmentInstanceID()
	store := &forwardCatalog{fakeStore: fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "stored-route"}}, instance: id}
	runtime := &forwardRuntime{fakeRuntime: fakeRuntime{status: core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}}}
	service := New(runtime, store)
	target, err := service.PrepareTCPForward(ctx, "demo", "::1", 8080)
	if err != nil || runtime.calls != 0 {
		t.Fatal(target, err, runtime.calls)
	}
	if _, err = service.DialTCPForward(ctx, target); !errors.Is(err, core.ErrRuntimeUnavailable) || runtime.calls != 1 || runtime.ref != "stored-route" || runtime.instance != id || runtime.address != "::1" || runtime.port != 8080 {
		t.Fatal(runtime, err)
	}
	other, _ := core.NewEnvironmentInstanceID()
	store.instance = other
	if _, err = service.DialTCPForward(ctx, target); !errors.Is(err, core.ErrCapabilityStale) || runtime.calls != 1 {
		t.Fatal(err, runtime.calls)
	}
	store.instance = id
	runtime.status.State = core.EnvironmentStopped
	if _, err = service.DialTCPForward(ctx, target); !errors.Is(err, core.ErrIncompatibleState) || runtime.calls != 1 {
		t.Fatal(err, runtime.calls)
	}
	runtime.status.State = core.EnvironmentRunning
	store.err = core.ErrRecoveryRequired
	if _, err = service.DialTCPForward(ctx, target); !errors.Is(err, core.ErrRecoveryRequired) || runtime.calls != 1 {
		t.Fatal(err, runtime.calls)
	}
}

func TestTCPForwardRejectsUnsafeTargetsBeforeStoreOrProvider(t *testing.T) {
	service := New(&forwardRuntime{}, &forwardCatalog{fakeStore: fakeStore{err: core.ErrNotFound}})
	for _, address := range []string{"localhost", "0.0.0.0", "10.0.0.1", "169.254.254.1", "::", "::1%lo", "::ffff:127.0.0.1", "-proxy"} {
		if _, err := service.PrepareTCPForward(context.Background(), "demo", address, 80); !errors.Is(err, core.ErrInvalidArgument) {
			t.Errorf("%s: %v", address, err)
		}
	}
	for _, port := range []int{-1, 0, 65536} {
		if _, err := service.PrepareTCPForward(context.Background(), "demo", "127.0.0.1", port); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"../demo", "--all", "", "haco demo"} {
		if _, err := service.PrepareTCPForward(context.Background(), name, "127.0.0.1", 80); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
}
