//go:build linux && (amd64 || arm64)

package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// StorageReclamation binds trusted local pool configuration to held native
// objects. It owns no Incus resource or mount and cannot create or delete one.
// The caller supplies installation configuration, never an untrusted RPC path.
// Its methods serialize native use and Close; Incus retains mount ownership.
type StorageReclamation struct {
	mu      sync.Mutex
	runtime *Runtime
	spec    BtrfsLoopPoolSpec
	source  string
	target  *pinnedReclaimTarget
}

func (r *Runtime) readReclaimPool(ctx context.Context, spec BtrfsLoopPoolSpec) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !diagnosticPoolName.MatchString(spec.Name) || spec.MountOptions == "" {
		return "", core.ErrInvalidArgument
	}
	// The installed local Incus layout is explicit. Do not let backend output
	// select a different Host directory or silently follow a custom layout.
	expected := "/var/lib/incus/disks/" + spec.Name + ".img"
	result, err := r.runner.Run(ctx, "incus", "storage", "list", "--project", sandboxResourceProject, "--format", "json")
	if err != nil {
		return "", fmt.Errorf("inspect configured reclaim pool: %w", err)
	}
	if result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		return "", core.ErrIncompatibleState
	}
	var pools []struct {
		Name   string            `json:"name"`
		Driver string            `json:"driver"`
		Status string            `json:"status"`
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &pools); err != nil {
		return "", core.ErrIncompatibleState
	}
	matches := 0
	for _, pool := range pools {
		if pool.Name != spec.Name {
			continue
		}
		matches++
		if pool.Driver != "btrfs" || pool.Status != "Created" || pool.Config["source"] != expected || pool.Config["btrfs.mount_options"] != spec.MountOptions {
			return "", core.ErrIncompatibleState
		}
	}
	if matches != 1 {
		return "", core.ErrIncompatibleState
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return expected, nil
}

// PrepareStorageReclamation inspects only the configured existing pool. A cold
// unmounted pool fails closed: its use/mount lifetime must be established by Incus.
func (r *Runtime) PrepareStorageReclamation(ctx context.Context, spec BtrfsLoopPoolSpec) (*StorageReclamation, error) {
	source, err := r.readReclaimPool(ctx, spec)
	if err != nil {
		return nil, err
	}
	target, err := pinReclaimTarget(spec.Name, source)
	if err != nil {
		return nil, err
	}
	operation := &StorageReclamation{runtime: r, spec: spec, source: source, target: target}
	if err := operation.validate(ctx); err != nil {
		return nil, errors.Join(err, operation.Close())
	}
	return operation, nil
}

func (s *StorageReclamation) validate(ctx context.Context) error {
	if s == nil || s.target == nil || s.runtime == nil {
		return core.ErrIncompatibleState
	}
	source, err := s.runtime.readReclaimPool(ctx, s.spec)
	if err != nil {
		return err
	}
	if source != s.source {
		return core.ErrIncompatibleState
	}
	return s.target.Validate()
}

func (s *StorageReclamation) TrimPool(ctx context.Context) (poolTrimObservation, error) {
	if s == nil {
		return poolTrimObservation{}, core.ErrIncompatibleState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validate(ctx); err != nil {
		return poolTrimObservation{}, err
	}
	return s.target.Trim(ctx)
}

// TrimBackingFilesystem additionally requires authority over the managed WSL
// distribution's outer filesystem. Permission for one pool is insufficient.
func (s *StorageReclamation) TrimBackingFilesystem(ctx context.Context) (outerTrimObservation, error) {
	if s == nil {
		return outerTrimObservation{}, core.ErrIncompatibleState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validate(ctx); err != nil {
		return outerTrimObservation{}, err
	}
	return s.target.TrimBackingFilesystem(ctx)
}

func (s *StorageReclamation) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.target == nil {
		return nil
	}
	target := s.target
	s.target = nil
	return target.Close()
}
