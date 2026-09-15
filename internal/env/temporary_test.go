package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestTemporaryWorkspaceRequiresProviderOptInBeforeCreate(t *testing.T) {
	provider := &fakeProvider{}
	router, err := NewRouter(testProvider, Register(testProvider, provider))
	if err != nil {
		t.Fatal(err)
	}
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	spec := core.EnvironmentRuntimeSpec{Name: "temp", WorkspacePath: work.Path, TemporaryWorkspace: true}
	for _, create := range []func(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error){router.CreateEnvironment, NewBaseRouter(router).CreateEnvironment} {
		if _, err := create(context.Background(), spec); !errors.Is(err, core.ErrUnsupported) {
			t.Fatal(err)
		}
	}
	if provider.created != 0 {
		t.Fatal("unsupported provider performed side effects")
	}
}
