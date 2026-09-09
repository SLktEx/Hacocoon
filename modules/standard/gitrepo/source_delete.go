package gitrepo

import (
	"context"
	"errors"
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
				if member.Repository == source.ID {
					v.Workspaces = append(v.Workspaces, w.ID)
					break
				}
			}
		}
		result = append(result, v)
	}
	return result, nil
}

// DeleteSource shares the registry lock with clone/copy. All Workspace records,
// including interrupted creations and deletions, keep their Git source alive.
func (s *RepositoryService) DeleteSource(ctx context.Context, id, owner string) error {
	if !ValidID(id) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: owner}) {
		return core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(interface {
		CheckSourceDeletion(context.Context, Object) error
		DeleteSourceVolume(context.Context, Object) error
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
	if source.State != "ready" && source.State != "deleting" {
		return core.ErrRecoveryRequired
	}
	work, err := s.listObjects(ctx, "work")
	if err != nil {
		return err
	}
	for _, w := range work {
		for _, member := range w.Copies() {
			if member.Repository == id {
				return core.ErrStorageBusy
			}
		}
	}
	if err := backend.CheckSourceDeletion(ctx, source); err != nil {
		return err
	}
	source.State = "deleting"
	if err := s.save(source); err != nil {
		return err
	}
	if err := backend.DeleteSourceVolume(ctx, source); err != nil {
		return errors.Join(core.ErrRecoveryRequired, err)
	}
	if err := os.Remove(s.path("repo", id)); err != nil {
		return err
	}
	return syncDir(s.Root)
}

// RunGit rechecks the exact source under the same registry lock as deletion and
// clone. A request that waited after approval cannot use a same-name replacement.
func (s *RepositoryService) RunGit(ctx context.Context, expected Object, req AgentRequest) (Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.Get("repo", expected.ID)
	if err != nil || expected.Kind != "repo" || !reflect.DeepEqual(current, expected) || req.Repository != expected.ID || req.Remote != expected.Remote || req.Branch != expected.Branch {
		return Response{}, core.ErrCapabilityStale
	}
	return s.Backend.RunGit(ctx, req)
}
