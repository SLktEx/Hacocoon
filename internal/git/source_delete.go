package gitrepo

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"reflect"
)

type SourceUse struct {
	Source     Object   `json:"source"`
	Workspaces []string `json:"workspaces"`
}

func (s *RepositoryService) ListSources(ctx context.Context) ([]SourceUse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sources, err := s.listObjects(ctx, "repo")
	if err != nil {
		return nil, err
	}
	work, err := s.listObjects(ctx, "work")
	if err != nil {
		return nil, err
	}
	result := []SourceUse{}
	for _, source := range sources {
		v := SourceUse{Source: source, Workspaces: []string{}}
		for _, w := range work {
			for _, member := range w.Copies() {
				if member.Remote != "" && member.Repository == source.ID {
					v.Workspaces = append(v.Workspaces, w.ID)
					break
				}
			}
		}
		result = append(result, v)
	}
	return result, nil
}

// DeleteSource removes the reviewed source after rechecking only the reviewed
// registry identity. Once the user confirms deletion, Workspace references,
// incomplete preparation, saved children and provider ownership/configuration
// preflights do not block the attempt. The exact managed native reference from
// the current source record remains the deletion target.
func (s *RepositoryService) DeleteSource(ctx context.Context, id, owner string) error {
	if !gitadapter.ValidID(id) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: owner}) {
		return core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(interface {
		ForceDeleteSourceVolume(context.Context, Object) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source, err := s.readObject("repo", id)
	if err != nil {
		return err
	}
	if source.Owner != owner {
		return core.ErrCapabilityStale
	}
	if err := backend.ForceDeleteSourceVolume(ctx, source); err != nil {
		return errors.Join(core.ErrRecoveryRequired, err)
	}
	if err := os.Remove(s.path("repo", id)); err != nil {
		return err
	}
	return syncDir(s.Root)
}

// ForceDeleteSource is the explicit recovery escape hatch. It keeps the
// registry lock so Git/source operations cannot race the removal, but skips
// source state, Workspace reference, native saved-object and ownership
// preflights. The provider still receives the exact managed native reference
// recorded for this source. A missing native source is treated as already
// deleted; provider mutation failure retains the registry record for retry.
func (s *RepositoryService) ForceDeleteSource(ctx context.Context, id string) error {
	if !gitadapter.ValidID(id) {
		return core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(interface {
		ForceDeleteSourceVolume(context.Context, Object) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source, err := s.readObject("repo", id)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := backend.ForceDeleteSourceVolume(ctx, source); err != nil {
		return errors.Join(core.ErrRecoveryRequired, err)
	}
	if err := os.Remove(s.path("repo", id)); err != nil {
		return err
	}
	return syncDir(s.Root)
}

// RunGit rechecks the exact source under the same registry lock as deletion and
// clone. A request that waited after approval cannot use a same-name replacement.
func (s *RepositoryService) RunGit(ctx context.Context, expected Object, req gitadapter.AgentRequest) (gitadapter.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.Get("repo", expected.ID)
	if err != nil || expected.Kind != "repo" || !reflect.DeepEqual(current, expected) || req.Repository != expected.ID || req.Remote != expected.Remote {
		return gitadapter.Response{}, core.ErrCapabilityStale
	}
	return s.Backend.RunGit(ctx, req)
}
