package gitrepo

import (
	"context"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// WorkspaceImport supplies data and explicit destination routing, not source
// authority. Archive readers must remain valid throughout the synchronous call.
type WorkspaceImport struct {
	Repository string
	Remote     string
	Branch     string
	Archive    io.ReadSeeker
}

// ImportWorkspaceSet uses the same all-member reservation/publication transition
// as normal Workspace copies, without running Git population. Partial collections
// retain their exact member receipts and cannot be leased, replayed or adopted.
func (s *RepositoryService) ImportWorkspaceSet(ctx context.Context, id string, inputs []WorkspaceImport) (Object, error) {
	if !ValidID(id) || len(inputs) < 2 || len(inputs) > 8 {
		return Object{}, core.ErrInvalidArgument
	}
	// Copy descriptors before planning; caller-owned slice entries are not state.
	inputs = append([]WorkspaceImport(nil), inputs...)
	seen := map[string]bool{}
	for _, input := range inputs {
		if !ValidID(input.Repository) || seen[input.Repository] ||
			!ValidWorkspaceRouting(input.Remote, input.Branch) || input.Archive == nil {
			return Object{}, core.ErrInvalidArgument
		}
		if input.Remote != "" && !strings.HasPrefix(input.Remote, "https://github.com/") {
			return Object{}, core.ErrUnsupported
		}
		seen[input.Repository] = true
	}
	backend, ok := s.Backend.(interface {
		ImportWorkspaceVolume(context.Context, Object, io.ReadSeeker) error
	})
	if !ok {
		return Object{}, core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object := Object{Kind: "work", ID: id, Owner: randomID(), State: "creating"}
	refs := map[string]bool{}
	for _, input := range inputs {
		owner := randomID()
		memberID := workspaceMemberID(id, input.Repository, owner)
		ref, err := s.Backend.Plan(ctx, "work", memberID)
		if err != nil {
			return Object{}, err
		}
		if ref == "" || refs[ref] {
			return Object{}, core.ErrIncompatibleState
		}
		refs[ref] = true
		object.Members = append(object.Members, Object{
			Kind: "work", ID: memberID, Repository: input.Repository, Remote: input.Remote,
			Branch: input.Branch, NativeRef: ref, Owner: owner, State: "creating",
		})
	}
	return s.createPreparedSet(ctx, object, func(ctx context.Context, i int, member Object) error {
		return backend.ImportWorkspaceVolume(ctx, member, inputs[i].Archive)
	}, nil)
}
