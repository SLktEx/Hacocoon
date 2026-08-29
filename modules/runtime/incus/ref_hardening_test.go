package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestStopRejectsAllOptionAsManagedRef(t *testing.T) {
	runner := &fakeRunner{}
	err := New(runner).Stop(context.Background(), "--all")
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("unsafe ref reached Incus: %#v", runner.calls)
	}
}

func TestDeleteEnvironmentRejectsRemoteQualifiedRef(t *testing.T) {
	runner := &fakeRunner{}
	err := New(runner).DeleteEnvironment(context.Background(), "evilremote:haco-demo")
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("remote-qualified ref reached Incus: %#v", runner.calls)
	}
}

func TestCreateEnvironmentRejectsRemoteLikeNameAtProviderBoundary(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "remote:demo",
		WorkspacePath: "/tmp/workspace",
	})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && call.args[0] == "init" {
			t.Fatalf("unsafe environment name reached Incus init: %#v", runner.calls)
		}
	}
}
