package gitrepo

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ValidateRepositorySelection accepts nil for the complete saved membership.
// An explicit selection must contain one to eight distinct logical names.
func ValidateRepositorySelection(repositories []string) error {
	if repositories == nil {
		return nil
	}
	if len(repositories) < 1 || len(repositories) > 8 {
		return core.ErrInvalidArgument
	}
	seen := map[string]bool{}
	for _, name := range repositories {
		if !ValidID(name) || seen[name] {
			return core.ErrInvalidArgument
		}
		seen[name] = true
	}
	return nil
}

type restoreSource struct {
	SavedWorkspace
	registered *Object
}

// Caller holds the repository registry lock through planning and publication.
// The complete destination record also pins added sources across interruption
// via the existing source-deletion reference checks. Never rewrite the snapshot:
// its full catalog identity remains the source reservation's authority.
func (s *RepositoryService) restoreSources(ctx context.Context, backend savedWorkspaceBackend, saved core.Snapshot, repositories []string) ([]restoreSource, error) {
	all, err := backend.SavedWorkspaces(ctx, saved)
	if err != nil {
		return nil, err
	}
	if len(all) < 1 || len(all) > 8 {
		return nil, core.ErrIncompatibleState
	}
	byName := map[string]SavedWorkspace{}
	for _, source := range all {
		if _, exists := byName[source.Repository]; exists || !ValidID(source.Repository) || !ValidWorkspaceRouting(source.Remote, source.Branch) || source.Component.State != "verified" {
			return nil, core.ErrIncompatibleState
		}
		byName[source.Repository] = source
	}
	if repositories == nil {
		for _, source := range all {
			repositories = append(repositories, source.Repository)
		}
	}
	sources := make([]restoreSource, 0, len(repositories))
	for _, name := range repositories {
		if source, exists := byName[name]; exists {
			sources = append(sources, restoreSource{SavedWorkspace: source})
			continue
		}
		repo, err := s.Get("repo", name)
		if err != nil {
			return nil, err
		}
		if err := s.Backend.InspectVolume(ctx, repo); err != nil {
			return nil, err
		}
		sources = append(sources, restoreSource{registered: &repo, SavedWorkspace: SavedWorkspace{
			Repository: name, Remote: repo.Remote, Branch: repo.Branch,
		}})
	}
	return sources, nil
}
