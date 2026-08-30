package btrfs

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block/localraw"
)

// NewLocal composes the supported local Btrfs storage implementation.
// The block-image seam is intentionally private to storage.btrfs; local storage
// uses one sparse raw image backend so Btrfs owns snapshot/COW semantics.
func NewLocal(ctx context.Context, rootDir string, runner host.Runner, requestedBackend string) (*Storage, error) {
	backend, err := chooseBlockBackend(ctx, runner, requestedBackend)
	if err != nil {
		return nil, err
	}
	return New(rootDir, backend, NewBtrfs(runner)), nil
}

func chooseBlockBackend(ctx context.Context, runner host.Runner, requested string) (block.Store, error) {
	if requested != "" && requested != "raw" {
		return nil, fmt.Errorf("unknown HACO_BLOCK_BACKEND %q; only raw is supported", requested)
	}

	raw := localraw.New(runner)
	caps, err := raw.Probe(ctx)
	if err != nil {
		return nil, err
	}
	if !caps.Available {
		return nil, fmt.Errorf("local raw block backend unavailable: %v", caps.Details)
	}
	return raw, nil
}
