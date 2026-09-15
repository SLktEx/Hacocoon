package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// DeleteTemporary shares canonical deletion and verifies the Workspace inside
// its lifecycle lock, so a recycled Environment name cannot change the target.
func (s *Service) DeleteTemporary(ctx context.Context, name string, work core.Workspace) error {
	if !core.ValidTemporaryWorkspace(work) {
		return core.ErrInvalidArgument
	}
	return s.delete(ctx, name, &work, "")
}

// DeleteRun keeps the generation check and deletion under the same lifecycle
// lock. A run marker or a run-* name cannot select a replacement Environment.
func (s *Service) DeleteRun(ctx context.Context, name, instance string) error {
	if !core.ValidEnvironmentInstanceID(instance) {
		return core.ErrInvalidArgument
	}
	return s.delete(ctx, name, nil, instance)
}
