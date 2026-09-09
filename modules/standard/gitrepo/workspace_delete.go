package gitrepo

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"strings"
)

// ListWorkspaces includes incomplete owned records so failures remain visible.
func (s *RepositoryService) ListWorkspaces(ctx context.Context) ([]Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.Root)
	if os.IsNotExist(err) {
		return []Object{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []Object{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "work-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "work-"), ".json")
		if !ValidID(id) || !entry.Type().IsRegular() {
			return nil, core.ErrIncompatibleState
		}
		o, err := s.readObject("work", id)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, nil
}

// DeleteWorkspace is called with the canonical Workspace lock held and all
// Environment leases excluded. Native ownership and absence are checked for
// each member. A partial delete retains the whole record for an explicit retry.
func (s *RepositoryService) DeleteWorkspace(ctx context.Context, id, owner string) error {
	if !ValidID(id) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: owner}) {
		return core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(interface {
		DeleteWorkspaceVolume(context.Context, Object) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object, err := s.readObject("work", id)
	if err != nil {
		return err
	}
	if object.Owner != owner {
		return core.ErrCapabilityStale
	}
	if object.State != "ready" && object.State != "deleting" {
		return core.ErrRecoveryRequired
	}
	object.State = "deleting"
	if err := s.save(object); err != nil {
		return err
	}
	for _, member := range object.Copies() {
		if err := backend.DeleteWorkspaceVolume(ctx, member); err != nil {
			return errors.Join(core.ErrRecoveryRequired, err)
		}
	}
	if err := os.Remove(s.path("work", id)); err != nil {
		return err
	}
	return syncDir(s.Root)
}
