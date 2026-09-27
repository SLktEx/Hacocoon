package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

// The deleted Environment retains ownership of its data until provider absence
// is established. This relation is moved atomically by FinalizeEnvironmentDelete.
func (s *EnvironmentJSONStore) OwnedWorkspaceCleanup(ctx context.Context, name string) (work core.Workspace, err error) {
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		var ok bool
		work, ok = data.OwnedWorkspaceCleanup[name]
		if !ok {
			return false, core.ErrNotFound
		}
		return false, nil
	})
	return
}
func (s *EnvironmentJSONStore) FinalizeOwnedWorkspaceCleanup(ctx context.Context, name string, work core.Workspace) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		current, ok := data.OwnedWorkspaceCleanup[name]
		if !ok {
			return false, nil
		}
		if current != work {
			return false, core.ErrCapabilityStale
		}
		delete(data.OwnedWorkspaceCleanup, name)
		return true, nil
	})
}

func validateOwnedWorkspaceCleanup(data environmentFileState) error {
	seen := map[core.WorkspaceID]bool{}
	for name, work := range data.OwnedWorkspaceCleanup {
		if core.ValidateEnvironmentName(name) != nil || !strings.HasPrefix(string(work.ID), "workspace:managed:") || !strings.HasPrefix(work.Path, "managed:") || seen[work.ID] {
			return core.ErrIncompatibleState
		}
		if _, ok := data.Environments[name]; ok {
			return core.ErrIncompatibleState
		}
		if _, ok := data.Leases[name]; ok {
			return core.ErrIncompatibleState
		}
		for _, lease := range data.Leases {
			if lease.WorkspaceID == work.ID || lease.SourcePath == work.Path {
				return core.ErrIncompatibleState
			}
		}
		seen[work.ID] = true
	}
	return nil
}
