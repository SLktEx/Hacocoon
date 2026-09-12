// Package workflow composes the existing retained-data and Env lifecycle APIs.
// It owns no second catalog and gives guests no management authority.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type Repositories interface {
	Get(string, string) (gitrepo.Object, error)
	CopyWorkspace(context.Context, string, string) (gitrepo.Object, error)
	CopyWorkspaceSet(context.Context, string, []string) (gitrepo.Object, error)
	RestoreWorkspaceWithData(context.Context, string, core.Snapshot, func(context.Context, core.Workspace) error) (gitrepo.Object, error)
}
type Environments interface {
	List(context.Context) ([]core.Environment, error)
	ListManagedWorkspaces(context.Context) ([]workspaceapp.ManagedWorkspace, error)
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	StartForWorkspace(context.Context, string, core.WorkspaceID) error
	CaptureStoppedSnapshotForWorkspace(context.Context, string, core.WorkspaceID) (core.Snapshot, error)
	DeleteSnapshot(context.Context, string) error
}
type Stores interface {
	RestoreSnapshot(context.Context, string, core.Snapshot, core.WorkspaceID) (core.PersistentResource, error)
}
type Service struct {
	Repositories Repositories
	Environments Environments
	Stores       Stores
}
type Reference struct {
	Name      string           `json:"name"`
	Workspace core.WorkspaceID `json:"workspace"`
}
type PrepareSpec struct {
	Name         string   `json:"name"`
	Repositories []string `json:"repositories"`
}
type OpenSpec struct {
	ExpectedResource core.PersistentResourceRef `json:"expected_resource,omitempty"`
	Reference
	Base core.BaseName `json:"base,omitempty"`
	OCI  string        `json:"oci,omitempty"` // auto, none, or an explicit oci: ID
}
type OpenResult struct {
	Reference
	Environment core.Environment `json:"environment"`
	Created     bool             `json:"created"`
}
type ForkResult struct {
	Resource core.PersistentResourceRef `json:"resource,omitempty"`
	Reference
	OCI               string        `json:"oci"`
	Base              core.BaseName `json:"base"`
	State             string        `json:"state"`
	TemporarySnapshot string        `json:"temporary_snapshot,omitempty"`
}

func reference(object gitrepo.Object) Reference {
	return Reference{Name: object.ID, Workspace: core.WorkspaceID("workspace:managed:" + object.Owner)}
}
func (s *Service) resolve(ref Reference) (gitrepo.Object, error) {
	if !gitrepo.ValidID(ref.Name) {
		return gitrepo.Object{}, core.ErrInvalidArgument
	}
	object, err := s.Repositories.Get("work", ref.Name)
	if err != nil {
		return object, err
	}
	if object.State != "ready" || object.Kind != "work" {
		return object, core.ErrRecoveryRequired
	}
	if ref.Workspace != "" && reference(object).Workspace != ref.Workspace {
		return object, core.ErrCapabilityStale
	}
	return object, nil
}
func (s *Service) Reference(_ context.Context, name string) (Reference, error) {
	object, err := s.resolve(Reference{Name: name})
	if err != nil {
		return Reference{}, err
	}
	return reference(object), nil
}

// Prepare is idempotent only for an already-ready copy with the same exact
// repository membership. It never retries or replaces an incomplete copy.
func (s *Service) Prepare(ctx context.Context, spec PrepareSpec) (Reference, error) {
	if !gitrepo.ValidID(spec.Name) || len(spec.Repositories) < 1 || len(spec.Repositories) > 8 {
		return Reference{}, core.ErrInvalidArgument
	}
	expected := append([]string(nil), spec.Repositories...)
	sort.Strings(expected)
	for i, id := range expected {
		if !gitrepo.ValidID(id) || i > 0 && id == expected[i-1] {
			return Reference{}, core.ErrInvalidArgument
		}
	}
	if current, err := s.Repositories.Get("work", spec.Name); err == nil {
		actual := []string{}
		for _, member := range current.Copies() {
			actual = append(actual, member.Repository)
		}
		sort.Strings(actual)
		if current.State != "ready" {
			return reference(current), core.ErrRecoveryRequired
		}
		if !reflect.DeepEqual(expected, actual) {
			return Reference{}, core.ErrAlreadyExists
		}
		return reference(current), nil
	} else if !errors.Is(err, core.ErrNotFound) {
		return Reference{}, err
	}
	var object gitrepo.Object
	var err error
	if len(spec.Repositories) == 1 {
		object, err = s.Repositories.CopyWorkspace(ctx, spec.Name, spec.Repositories[0])
	} else {
		object, err = s.Repositories.CopyWorkspaceSet(ctx, spec.Name, spec.Repositories)
	}
	if err != nil {
		if object.ID != "" {
			return reference(object), err
		}
		return Reference{}, err
	}
	return reference(object), nil
}
func (s *Service) Open(ctx context.Context, spec OpenSpec) (OpenResult, error) {
	if spec.Workspace == "" || (spec.OCI != "" && spec.OCI != "auto" && spec.OCI != "none" && !strings.HasPrefix(spec.OCI, "oci:")) {
		return OpenResult{}, core.ErrInvalidArgument
	}
	if spec.ExpectedResource != (core.PersistentResourceRef{}) && (!core.ValidPersistentResourceRef(spec.ExpectedResource) || spec.OCI == "none" || (strings.HasPrefix(spec.OCI, "oci:") && spec.OCI != spec.ExpectedResource.ID)) {
		return OpenResult{}, core.ErrInvalidArgument
	}
	object, err := s.resolve(spec.Reference)
	if err != nil {
		return OpenResult{}, err
	}
	ref := reference(object)
	result := OpenResult{Reference: ref}
	all, err := s.Environments.List(ctx)
	if err != nil {
		return result, err
	}
	matches := []core.Environment{}
	for _, env := range all {
		if env.Workspace.ID == ref.Workspace {
			matches = append(matches, env)
		}
	}
	if len(matches) > 1 {
		return result, fmt.Errorf("multiple Envs reference this work; select and resolve ownership explicitly: %w", core.ErrStorageBusy)
	}
	if len(matches) == 1 {
		return s.openExisting(ctx, spec, result, matches[0])
	}
	envSpec := core.EnvironmentSpec{ExpectedWorkspace: ref.Workspace, Name: "work-" + object.ID, WorkspacePath: "managed:" + object.ID, Base: spec.Base, SkipDefaultResource: spec.OCI == "none"}
	if strings.HasPrefix(spec.OCI, "oci:") {
		envSpec.PersistentResource = spec.OCI
	}
	if spec.ExpectedResource != (core.PersistentResourceRef{}) {
		envSpec.PersistentResource = spec.ExpectedResource.ID
		envSpec.ExpectedResource = spec.ExpectedResource
	}
	env, err := s.Environments.Create(ctx, envSpec)
	if err != nil {
		// Another concurrent open may have completed canonical creation. Only
		// adopt the exact work and matching options; acquiring/failed leases
		// remain errors and are never repaired here.
		if errors.Is(err, core.ErrAlreadyExists) {
			current, listErr := s.Environments.List(ctx)
			if listErr != nil {
				return result, listErr
			}
			for _, candidate := range current {
				if candidate.Name == envSpec.Name && candidate.Workspace.ID == ref.Workspace {
					return s.openExisting(ctx, spec, result, candidate)
				}
			}
		}
		return result, err
	}
	if env.Workspace.ID != ref.Workspace {
		return result, core.ErrCapabilityStale
	}
	result.Environment, result.Created = env, true
	return result, nil
}
func (s *Service) openExisting(ctx context.Context, spec OpenSpec, result OpenResult, env core.Environment) (OpenResult, error) {
	if spec.ExpectedResource != (core.PersistentResourceRef{}) && env.PersistentResource != spec.ExpectedResource {
		return result, core.ErrCapabilityStale
	}
	if env.AccessMode != core.WorkspaceReadWrite || (spec.Base != "" && (env.Base == nil || env.Base.Name != spec.Base)) ||
		(spec.OCI == "none" && env.PersistentResource.ID != "") ||
		(strings.HasPrefix(spec.OCI, "oci:") && env.PersistentResource.ID != spec.OCI) {
		return result, core.ErrIncompatibleState
	}
	if err := s.Environments.StartForWorkspace(ctx, env.Name, spec.Workspace); err != nil {
		return result, err
	}
	result.Environment = env
	return result, nil
}

// Fork captures the stopped aggregate through its canonical API, then reuses
// existing COW data restores. Rootfs is captured for existing snapshot semantics
// but never restored. Failed data copies retain their owning catalog records.
func (s *Service) Fork(ctx context.Context, source Reference, target string) (result ForkResult, err error) {
	if source.Workspace == "" || !gitrepo.ValidID(target) || source.Name == target {
		return result, core.ErrInvalidArgument
	}
	object, err := s.resolve(source)
	if err != nil {
		return result, err
	}
	if _, err := s.Repositories.Get("work", target); err == nil {
		return result, core.ErrAlreadyExists
	} else if !errors.Is(err, core.ErrNotFound) {
		return result, err
	}
	work := reference(object)
	all, err := s.Environments.ListManagedWorkspaces(ctx)
	if err != nil {
		return result, err
	}
	var names []string
	for _, w := range all {
		if w.Workspace.ID == work.Workspace {
			names = w.Environments
		}
	}
	if len(names) != 1 {
		return result, fmt.Errorf("fork requires one stopped Env for this work; reopen detached work, then stop it: %w", core.ErrIncompatibleState)
	}
	saved, err := s.Environments.CaptureStoppedSnapshotForWorkspace(ctx, names[0], work.Workspace)
	result.TemporarySnapshot = saved.ID
	if saved.ID != "" {
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if e := s.Environments.DeleteSnapshot(cleanup, saved.ID); e != nil {
				err = errors.Join(err, e, core.ErrRecoveryRequired)
			} else {
				result.TemporarySnapshot = ""
			}
		}()
	}
	if err != nil {
		return result, err
	}
	if saved.Source.Environment.Base != nil {
		result.Base = saved.Source.Environment.Base.Name
	}
	result.OCI = "none"
	copied, err := s.Repositories.RestoreWorkspaceWithData(ctx, target, saved, func(ctx context.Context, work core.Workspace) error {
		result.Reference = Reference{Name: target, Workspace: work.ID}
		if saved.Source.Environment.PersistentResource.ID == "" {
			return nil
		}
		if s.Stores == nil {
			return core.ErrUnsupported
		}
		resource, e := s.Stores.RestoreSnapshot(ctx, "oci:"+target, saved, work.ID)
		if resource.ID != "" {
			result.OCI = resource.ID
			result.Resource = resource.Ref()
		}
		return e
	})
	if copied.ID != "" {
		result.Reference = reference(copied)
	}
	if err != nil {
		result.State = "recovery-required"
		return result, err
	}
	result.State = "ready"
	return result, nil
}
