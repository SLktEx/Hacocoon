package workspace

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// CleanupRestoredData excludes canonical create/delete for this fresh managed
// Workspace while checking durable leases and removing exact-owned copies.
// A caller supplies owned cleanup, never a guessed path or Environment deletion.
func (s *Service) CleanupRestoredData(ctx context.Context, work core.Workspace, remove func(context.Context) error) error {
	if work.ID == "" || !strings.HasPrefix(work.Path, "managed:") || remove == nil {
		return core.ErrInvalidArgument
	}
	leases, ok := s.store.(interface {
		ListWorkspaceLeases(context.Context) ([]core.WorkspaceLease, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	unlock, err := lockWorkspace(ctx, work.ID)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := s.provider.Resolve(ctx, WorkspaceRequest{Path: work.Path})
	if err != nil {
		return err
	}
	if current.ID != work.ID || current.Path != work.Path {
		return core.ErrCapabilityStale
	}
	all, err := leases.ListWorkspaceLeases(ctx)
	if err != nil {
		return err
	}
	for _, lease := range all {
		if lease.WorkspaceID == work.ID || lease.SourcePath == work.Path {
			return core.ErrStorageBusy
		}
	}
	return remove(ctx)
}
