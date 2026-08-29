package composition

import (
	"context"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs"
)

type App struct {
	Manager *core.Manager
	Runtime core.Runtime
	Storage core.Storage
}

func Local(ctx context.Context) (*App, error) {
	runner := host.ExecRunner{}
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	storage, err := btrfs.NewLocal(ctx, filepath.Join(root, "storage"), runner, os.Getenv("HACO_BLOCK_BACKEND"))
	if err != nil {
		return nil, err
	}
	runtime := incus.New(runner)
	store := state.NewJSONStore(filepath.Join(root, "state", "sessions.json"))
	return &App{
		Manager: core.NewManager(runtime, storage, store),
		Runtime: runtime,
		Storage: storage,
	}, nil
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
