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
	return s.delete(ctx, name, &work)
}
