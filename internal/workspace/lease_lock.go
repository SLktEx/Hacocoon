package workspace

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *Service) lockWorkspace(ctx context.Context, id core.WorkspaceID) (func(), error) {
	return s.lockLifecycle(ctx, "workspace", string(id))
}

func (s *Service) lockLifecycle(ctx context.Context, domain, id string) (func(), error) {
	return s.store.LockLifecycle(ctx, domain, id)
}
