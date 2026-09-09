package oci

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"sort"
)

type StoreUse struct {
	Resource             core.PersistentResourceRef `json:"resource"`
	Role                 string                     `json:"role"`
	Environments         []string                   `json:"environments"`
	IndependentSnapshots []string                   `json:"independent_snapshots"`
	PendingCopies        []string                   `json:"pending_copies"`
}
type StoreReferenceCatalog interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
	ListWorkspaceLeases(context.Context) ([]core.WorkspaceLease, error)
	ListSnapshots(context.Context) ([]core.Snapshot, error)
}

// StoreUses projects existing references; it creates no additional catalog.
func StoreUses(ctx context.Context, catalog StoreReferenceCatalog, resources []core.PersistentResource) ([]StoreUse, error) {
	envs, err := catalog.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	leases, err := catalog.ListWorkspaceLeases(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := catalog.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	uses := []StoreUse{}
	for _, r := range resources {
		if r.Kind != StoreKind {
			continue
		}
		v := StoreUse{Resource: r.Ref(), Role: "independent-store", Environments: []string{}, IndependentSnapshots: []string{}, PendingCopies: []string{}}
		if r.SourceOnly {
			v.Role = "host-source"
		}
		names := map[string]bool{}
		for _, e := range envs {
			if e.PersistentResource.ID == r.ID {
				names[e.Name] = true
			}
		}
		for _, l := range leases {
			if l.PersistentResource.ID == r.ID {
				names[l.EnvironmentID] = true
			}
		}
		for name := range names {
			v.Environments = append(v.Environments, name)
		}
		for _, saved := range snapshots {
			if saved.Source.Environment.PersistentResource == r.Ref() {
				v.IndependentSnapshots = append(v.IndependentSnapshots, saved.ID)
			}
		}
		for _, copy := range resources {
			if copy.CopySource == r.Ref() {
				v.PendingCopies = append(v.PendingCopies, copy.ID)
			}
		}
		sort.Strings(v.Environments)
		sort.Strings(v.IndependentSnapshots)
		sort.Strings(v.PendingCopies)
		uses = append(uses, v)
	}
	return uses, nil
}
