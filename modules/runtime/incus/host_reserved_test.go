package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCreateEnvironmentRejectsTrustedHostNameCollision(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "host",
		WorkspacePath: "/tmp/workspace",
	})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("reserved name touched Incus: %#v", runner.calls)
	}
}
