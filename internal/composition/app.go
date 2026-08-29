package composition

import (
	"context"
	"os"
	"path/filepath"

	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type App struct {
	Environments *workspaceapp.Service
	Clients      *clientapp.Service
	Runtime      *incus.Runtime
}

func Local(_ context.Context) (*App, error) {
	runner := host.ExecRunner{}
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	runtime := incus.New(runner)
	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	return &App{
		Environments: workspaceapp.New(runtime, store),
		Clients:      clientapp.New(runtime, store),
		Runtime:      runtime,
	}, nil
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
