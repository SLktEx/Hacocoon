package composition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
	"github.com/SLktEx/Hacocoon/modules/storage/block"
	"github.com/SLktEx/Hacocoon/modules/storage/block/localqcow2"
	"github.com/SLktEx/Hacocoon/modules/storage/block/localraw"
	"github.com/SLktEx/Hacocoon/modules/storage/localbtrfs"
)

type App struct {
	Manager *core.Manager
	Runtime core.Runtime
	Storage core.Storage
}

func Local(ctx context.Context) (*App, error) {
	runner := host.ExecRunner{}
	backend, err := chooseBlockBackend(ctx, runner)
	if err != nil {
		return nil, err
	}
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	storage := localbtrfs.New(filepath.Join(root, "storage"), backend, localbtrfs.NewBtrfs(runner))
	runtime := incus.New(runner)
	store := state.NewJSONStore(filepath.Join(root, "state", "sessions.json"))
	return &App{
		Manager: core.NewManager(runtime, storage, store),
		Runtime: runtime,
		Storage: storage,
	}, nil
}

func chooseBlockBackend(ctx context.Context, runner host.Runner) (block.Store, error) {
	qcow := localqcow2.New(runner)
	raw := localraw.New(runner)
	strategies := map[string]block.Store{
		"qcow2": qcow,
		"raw":   raw,
	}
	requested := os.Getenv("HACO_BLOCK_BACKEND")
	if requested != "" {
		backend, ok := strategies[requested]
		if !ok {
			return nil, fmt.Errorf("unknown HACO_BLOCK_BACKEND %q", requested)
		}
		caps, err := backend.Probe(ctx)
		if err != nil {
			return nil, err
		}
		if !caps.Available {
			return nil, fmt.Errorf("requested block backend %s unavailable: %v", requested, caps.Details)
		}
		return backend, nil
	}
	qcowCaps, err := qcow.Probe(ctx)
	if err == nil && qcowCaps.Available {
		return qcow, nil
	}
	rawCaps, rawErr := raw.Probe(ctx)
	if rawErr != nil {
		return nil, rawErr
	}
	if rawCaps.Available {
		return raw, nil
	}
	return nil, fmt.Errorf("no supported local block backend; qcow2=%v raw=%v", qcowCaps.Details, rawCaps.Details)
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
