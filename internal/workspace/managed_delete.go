package workspace

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"sort"
	"strings"
)

type ManagedWorkspace struct {
	Workspace    core.Workspace `json:"workspace"`
	Name         string         `json:"name"`
	State        string         `json:"state"`
	Repositories []string       `json:"repositories"`
	Environments []string       `json:"environments"`
	Snapshots    []string       `json:"independent_snapshots"`
	Stores       []string       `json:"retained_oci_stores"`
}

type managedWorkspaceCatalog interface {
	ListWorkspaceLeases(context.Context) ([]core.WorkspaceLease, error)
	ListEnvironments(context.Context) ([]core.Environment, error)
	ListSnapshots(context.Context) ([]core.Snapshot, error)
	ListPersistentResources(context.Context) ([]core.PersistentResource, error)
}

func (s *Service) ListManagedWorkspaces(ctx context.Context) ([]ManagedWorkspace, error) {
	provider, ok := s.provider.(interface {
		ListManagedWorkspaces(context.Context) ([]ManagedWorkspace, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	catalog, ok := s.store.(managedWorkspaceCatalog)
	if !ok {
		return nil, core.ErrUnsupported
	}
	all, err := provider.ListManagedWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	leases, err := catalog.ListWorkspaceLeases(ctx)
	if err != nil {
		return nil, err
	}
	envs, err := catalog.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := catalog.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	resources, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		w := &all[i]
		names := map[string]bool{}
		for _, l := range leases {
			if l.WorkspaceID == w.Workspace.ID || l.SourcePath == w.Workspace.Path {
				names[l.EnvironmentID] = true
			}
		}
		for _, e := range envs {
			if e.Workspace.ID == w.Workspace.ID || e.Workspace.Path == w.Workspace.Path {
				names[e.Name] = true
			}
		}
		w.Environments = []string{}
		w.Snapshots = []string{}
		w.Stores = []string{}
		for n := range names {
			w.Environments = append(w.Environments, n)
		}
		sort.Strings(w.Environments)
		for _, saved := range snapshots {
			if saved.Source.Environment.Workspace.ID == w.Workspace.ID {
				w.Snapshots = append(w.Snapshots, saved.ID)
			}
		}
		for _, r := range resources {
			if r.WorkspaceID == w.Workspace.ID {
				w.Stores = append(w.Stores, r.ID)
			}
		}
	}
	return all, nil
}

// DeleteManagedWorkspace uses the same lock as create/capture and checks both
// committed Environments and all intermediate leases. The provider rechecks
// the expected owner and durably refuses new consumers before native deletion.
func (s *Service) DeleteManagedWorkspace(ctx context.Context, work core.Workspace) error {
	if !strings.HasPrefix(work.Path, "managed:") || !strings.HasPrefix(string(work.ID), "workspace:managed:") {
		return core.ErrInvalidArgument
	}
	provider, ok := s.provider.(interface {
		DeleteWorkspace(context.Context, core.Workspace) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	catalog, ok := s.store.(managedWorkspaceCatalog)
	if !ok {
		return core.ErrUnsupported
	}
	unlock, err := s.lockWorkspace(ctx, work.ID)
	if err != nil {
		return err
	}
	defer unlock()
	leases, err := catalog.ListWorkspaceLeases(ctx)
	if err != nil {
		return err
	}
	for _, l := range leases {
		if l.WorkspaceID == work.ID || l.SourcePath == work.Path {
			return core.ErrStorageBusy
		}
	}
	envs, err := catalog.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	for _, e := range envs {
		if e.Workspace.ID == work.ID || e.Workspace.Path == work.Path {
			return core.ErrStorageBusy
		}
	}
	return provider.DeleteWorkspace(ctx, work)
}

// DeleteOwnedWorkspace removes only resources durably bound to this exact
// Workspace. Each resource transition independently excludes current consumers.
func (s *Service) DeleteOwnedWorkspace(ctx context.Context, work core.Workspace) error {
	catalog, ok := s.store.(managedWorkspaceCatalog)
	if !ok {
		return core.ErrUnsupported
	}
	all, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range all {
		if resource.WorkspaceID != work.ID {
			continue
		}
		if s.deleteWorkspaceResource == nil {
			return core.ErrUnsupported
		}
		if err := s.deleteWorkspaceResource(ctx, resource.ID, work.ID); err != nil {
			return fmt.Errorf("cleanup Workspace %s resource %s: %w", work.Path, resource.ID, err)
		}
	}
	if err := s.DeleteManagedWorkspace(ctx, work); err != nil {
		return fmt.Errorf("cleanup Workspace %s: %w", work.Path, err)
	}
	return nil
}
