package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SavedWorkspace is trusted catalog metadata, never guest Git configuration.
// Component retains the exact immutable provider source receipt.
type SavedWorkspace struct {
	Component                  core.SnapshotComponent
	Repository, Remote, Branch string
}

// SnapshotWorkspaceCatalog serializes source deletion against native copies.
// The registry remains responsible for destination ownership and cleanup.
type SnapshotWorkspaceCatalog interface {
	BeginSnapshotWorkspaceCopy(context.Context, core.Snapshot, string, string) error
	FinishSnapshotWorkspaceCopy(context.Context, string, string, string) error
}

type savedWorkspaceBackend interface {
	SavedWorkspaces(context.Context, core.Snapshot) ([]SavedWorkspace, error)
	PlanSavedWorkspace(context.Context, string, SavedWorkspace) (string, error)
	CreateSavedWorkspace(context.Context, Object, SavedWorkspace) error
	DeleteRestoredWorkspaceVolume(context.Context, Object) error
}

func validSavedID(id string) bool {
	return strings.HasPrefix(id, "snap-") && len(id) == 37 && core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: strings.TrimPrefix(id, "snap-")})
}

// RestoreWorkspace registers independent normal Workspace copies. The caller
// supplies the catalog used for saved-data deletion. This service holds the
// source reservation until publication or positive cleanup. No source
// checkout, Git network operation, old Base or old Environment is required.
func (s *RepositoryService) RestoreWorkspace(ctx context.Context, id string, saved core.Snapshot) (Object, error) {
	if !ValidID(id) || !validSavedID(saved.ID) || saved.State != "ready" {
		return Object{}, core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(savedWorkspaceBackend)
	if !ok || s.SnapshotCatalog == nil {
		return Object{}, core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sources, err := backend.SavedWorkspaces(ctx, saved)
	if err != nil {
		return Object{}, err
	}
	if len(sources) < 1 || len(sources) > 8 {
		return Object{}, core.ErrIncompatibleState
	}
	object := Object{Kind: "work", ID: id, Owner: randomID(), State: "creating", RestoredFrom: saved.ID}
	seen := map[string]bool{}
	for _, source := range sources {
		if !ValidID(source.Repository) || ValidateRemote(source.Remote) != nil || !ValidBranch(source.Branch) || seen[source.Repository] || source.Component.State != "verified" {
			return Object{}, core.ErrIncompatibleState
		}
		seen[source.Repository] = true
		member := Object{Kind: "work", ID: id, Repository: source.Repository, Remote: source.Remote, Branch: source.Branch, Owner: object.Owner, State: "creating", RestoredFrom: saved.ID}
		if len(sources) > 1 {
			member.ID = id + "-" + source.Repository
			member.Owner = randomID()
		}
		if !ValidID(member.ID) {
			return Object{}, core.ErrInvalidArgument
		}
		member.NativeRef, err = backend.PlanSavedWorkspace(ctx, member.ID, source)
		if err != nil {
			return Object{}, err
		}
		if len(sources) == 1 {
			object = member
		} else {
			object.Members = append(object.Members, member)
		}
	}
	if !validObject(object) {
		return Object{}, core.ErrIncompatibleState
	}
	if err := s.reserve(object); err != nil {
		return Object{}, err
	}
	fail := func(cause error) (Object, error) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		object.State = "creating"
		markErr := s.save(object)
		if cleanupErr := s.cleanupRestoredWorkspace(cleanup, backend, object); cleanupErr != nil {
			return object, fmt.Errorf("restored workspace %s cleanup incomplete: %w", id, errors.Join(cause, markErr, cleanupErr, core.ErrRecoveryRequired))
		}
		// This operation failed even when all newly owned copies are now absent.
		if errors.Is(cause, core.ErrRecoveryRequired) {
			cause = core.ErrRuntimeUnavailable
		}
		return Object{}, fmt.Errorf("restored workspace %s failed; new copies removed: %w", id, cause)
	}
	if err := s.SnapshotCatalog.BeginSnapshotWorkspaceCopy(ctx, saved, object.ID, object.Owner); err != nil {
		return fail(err)
	}
	for i, source := range sources {
		member := &object
		if len(object.Members) != 0 {
			member = &object.Members[i]
		}
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := backend.CreateSavedWorkspace(ctx, *member, source); err != nil {
			return fail(err)
		}
		member.State = "created"
		if err := s.save(object); err != nil {
			return fail(err)
		}
		if err := s.Backend.InspectVolume(ctx, *member); err != nil {
			return fail(err)
		}
		member.State = "ready"
	}
	object.State = "ready"
	if err := s.save(object); err != nil {
		return fail(err)
	}
	if err := s.SnapshotCatalog.FinishSnapshotWorkspaceCopy(ctx, saved.ID, object.ID, object.Owner); err != nil {
		return object, fmt.Errorf("workspace %s is ready; source reservation release failed: %w", id, errors.Join(err, core.ErrRecoveryRequired))
	}
	return object, nil
}

// CleanupRestoredWorkspace removes incomplete restored copies or retries a
// pending source release for a published copy. It never deletes published data.
func (s *RepositoryService) CleanupRestoredWorkspace(ctx context.Context, id string) error {
	backend, ok := s.Backend.(savedWorkspaceBackend)
	if !ok || s.SnapshotCatalog == nil {
		return core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object, err := s.Get("work", id)
	if err == nil && validSavedID(object.RestoredFrom) {
		// Retry only the pending release; never delete published data.
		err = s.SnapshotCatalog.FinishSnapshotWorkspaceCopy(ctx, object.RestoredFrom, object.ID, object.Owner)
		if errors.Is(err, core.ErrNotFound) {
			return core.ErrIncompatibleState
		}
		return err
	}
	if !errors.Is(err, core.ErrRecoveryRequired) || !validSavedID(object.RestoredFrom) {
		if err != nil {
			return err
		}
		return core.ErrIncompatibleState
	}
	return s.cleanupRestoredWorkspace(ctx, backend, object)
}

func (s *RepositoryService) cleanupRestoredWorkspace(ctx context.Context, backend savedWorkspaceBackend, object Object) error {
	var failures []error
	for _, member := range object.Copies() {
		if err := backend.DeleteRestoredWorkspaceVolume(ctx, member); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	if err := s.SnapshotCatalog.FinishSnapshotWorkspaceCopy(ctx, object.RestoredFrom, object.ID, object.Owner); err != nil && !errors.Is(err, core.ErrNotFound) {
		return err
	}
	if err := os.Remove(s.path("work", object.ID)); err != nil {
		return err
	}
	return syncDir(s.Root)
}
