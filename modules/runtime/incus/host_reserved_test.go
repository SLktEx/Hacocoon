package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCreateEnvironmentRejectsReservedHostNameBeforeProviderMutation(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "host", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("provider mutated before reserved-name rejection: %#v", runner.calls)
	}
}
