package btrfs

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block/localqcow2"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block/localraw"
)

// NewLocal composes the supported local Btrfs storage implementation.
// The block-image seam is intentionally private to storage.btrfs; callers only
// select an optional backend preference and receive the Core-facing Storage.
func NewLocal(ctx context.Context, rootDir string, runner host.Runner, requestedBackend string) (*Storage, error) {
	backend, err := chooseBlockBackend(ctx, runner, requestedBackend)
	if err != nil {
		return nil, err
	}
	return New(rootDir, backend, NewBtrfs(runner)), nil
}

func chooseBlockBackend(ctx context.Context, runner host.Runner, requested string) (block.Store, error) {
	qcow := localqcow2.New(runner)
	raw := localraw.New(runner)
	strategies := map[string]block.Store{
		"qcow2": qcow,
		"raw":   raw,
	}

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

	qcowCaps, qcowErr := qcow.Probe(ctx)
	if qcowErr == nil && qcowCaps.Available {
		return qcow, nil
	}

	rawCaps, rawErr := raw.Probe(ctx)
	if rawErr != nil {
		return nil, rawErr
	}
	if rawCaps.Available {
		return raw, nil
	}

	qcowDetails := qcowCaps.Details
	if qcowErr != nil {
		qcowDetails = []string{qcowErr.Error()}
	}
	return nil, fmt.Errorf("no supported local block backend; qcow2=%v raw=%v", qcowDetails, rawCaps.Details)
}
