package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
)

// ImportWorkspace registers one independent archive without cloning, checking out
// or executing guest Git configuration. The public aggregate importer is separate.
// Local source remotes and absent routing require an explicit offline/rebinding
// policy; this initial registration primitive does not adopt destination Host paths.
func (s *RepositoryService) ImportWorkspace(ctx context.Context, id, repository, remote, branch string, archive io.ReadSeeker) (Object, error) {
	if !ValidID(id) || !ValidID(repository) || !ValidBranch(branch) || ValidateRemote(remote) != nil || archive == nil {
		return Object{}, core.ErrInvalidArgument
	}
	if !strings.HasPrefix(remote, "https://github.com/") {
		return Object{}, core.ErrUnsupported
	}
	backend, ok := s.Backend.(interface {
		ImportWorkspaceVolume(context.Context, Object, io.ReadSeeker) error
	})
	if !ok {
		return Object{}, core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object := Object{Kind: "work", ID: id, Repository: repository, Remote: remote, Branch: branch}
	created, err := s.createPrepared(ctx, object, func(ctx context.Context, o Object) error { return backend.ImportWorkspaceVolume(ctx, o, archive) }, nil)
	if err == nil || created.State != "created" {
		return created, err
	}
	return s.cleanupFailedImport(ctx, created, err)
}

// Called only by the failed invocation, under the same service lock. There is no
// public state-based cleanup entry that could race an active creator. "created"
// proves native completion; "creating" may still have an unresolved native task,
// and "ready" may already be visible after an ambiguous publication failure.
func (s *RepositoryService) cleanupFailedImport(ctx context.Context, expected Object, cause error) (Object, error) {
	if expected.Kind != "work" || expected.State != "created" || expected.RestoredFrom != "" || len(expected.Members) != 0 || !validObject(expected) {
		return expected, errors.Join(cause, core.ErrInvalidArgument, core.ErrRecoveryRequired)
	}
	backend, ok := s.Backend.(interface {
		DeleteWorkspaceVolume(context.Context, Object) error
	})
	if !ok {
		return expected, cause
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	current, err := s.readObject("work", expected.ID)
	if err != nil {
		return expected, errors.Join(cause, err, core.ErrRecoveryRequired)
	}
	if !reflect.DeepEqual(current, expected) {
		return expected, errors.Join(cause, core.ErrCapabilityStale, core.ErrRecoveryRequired)
	}
	// The native deletion contract checks ownership, users and saved children,
	// and returns success only after independent positive absence verification.
	if err := backend.DeleteWorkspaceVolume(cleanup, current); err != nil {
		return expected, errors.Join(cause, err, core.ErrRecoveryRequired)
	}
	if err := os.Remove(s.path("work", expected.ID)); err != nil {
		return expected, errors.Join(cause, err, core.ErrRecoveryRequired)
	}
	if err := syncDir(s.Root); err != nil {
		return expected, errors.Join(cause, err, core.ErrRecoveryRequired)
	}
	// Cleanup completion does not turn the import into a successful operation.
	return Object{}, fmt.Errorf("Workspace import failed; new owned volume removed (%v): %w", cause, core.ErrRuntimeUnavailable)
}
