package state

import (
	"context"
	"fmt"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Catalog and attachment reservations share the same durable transaction.
func (s *EnvironmentJSONStore) resourceTransaction(fn func(*environmentFileState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()
	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if err := fn(&data); err != nil {
		return err
	}
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) ListPersistentResources(context.Context) ([]core.PersistentResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	data, err := s.readEnvironments()
	if err != nil {
		return nil, err
	}
	list := make([]core.PersistentResource, 0, len(data.PersistentResources))
	for _, r := range data.PersistentResources {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (s *EnvironmentJSONStore) GetPersistentResource(ctx context.Context, id string) (core.PersistentResource, error) {
	list, err := s.ListPersistentResources(ctx)
	if err != nil {
		return core.PersistentResource{}, err
	}
	for _, r := range list {
		if r.ID == id {
			return r, nil
		}
	}
	return core.PersistentResource{}, core.ErrNotFound
}

func (s *EnvironmentJSONStore) BeginPersistentResourceCreate(_ context.Context, r core.PersistentResource) error {
	if !core.ValidPersistentResourceRef(r.Ref()) || r.Kind == "" || r.NativeRef == "" || r.State != "creating" || r.CreatedAt.IsZero() || r.CopySource != (core.PersistentResourceRef{}) {
		return core.ErrInvalidArgument
	}
	return s.resourceTransaction(func(d *environmentFileState) error {
		if _, ok := d.PersistentResources[r.ID]; ok {
			return core.ErrAlreadyExists
		}
		d.PersistentResources[r.ID] = r
		return nil
	})
}

func (s *EnvironmentJSONStore) CommitPersistentResourceCreate(_ context.Context, r core.PersistentResource) error {
	return s.resourceTransaction(func(d *environmentFileState) error {
		if existing, ok := d.PersistentResources[r.ID]; !ok || existing != r || r.State != "creating" {
			return core.ErrIncompatibleState
		}
		r.State = "ready"
		r.CopySource = core.PersistentResourceRef{}
		d.PersistentResources[r.ID] = r
		return nil
	})
}

func (s *EnvironmentJSONStore) BeginPersistentResourceDelete(_ context.Context, id string) (r core.PersistentResource, err error) {
	err = s.resourceTransaction(func(d *environmentFileState) error {
		var ok bool
		r, ok = d.PersistentResources[id]
		if !ok {
			return core.ErrNotFound
		}
		if r.CopySource != (core.PersistentResourceRef{}) {
			return core.ErrRecoveryRequired
		}
		if persistentCopyReserved(*d, id) {
			return core.ErrStorageBusy
		}
		// A missing/old lease is not evidence that a committed attachment vanished.
		for _, e := range d.Environments {
			if e.PersistentResource.ID == id {
				return core.ErrStorageBusy
			}
		}
		for _, l := range d.Leases {
			if l.PersistentResource.ID == id {
				return core.ErrStorageBusy
			}
		}
		if r.State != "ready" && r.State != "creating" && r.State != "deleting" {
			return core.ErrRecoveryRequired
		}
		r.State = "deleting"
		d.PersistentResources[id] = r
		return nil
	})
	return
}

// Only the provider-owning service may finalize after positively proving absence.
func (s *EnvironmentJSONStore) FinalizePersistentResourceDelete(_ context.Context, r core.PersistentResource) error {
	return s.resourceTransaction(func(d *environmentFileState) error {
		if existing, ok := d.PersistentResources[r.ID]; !ok || existing != r || r.State != "deleting" {
			return fmt.Errorf("persistent resource ownership changed: %w", core.ErrIncompatibleState)
		}
		for _, l := range d.Leases {
			if l.PersistentResource.ID == r.ID {
				return core.ErrStorageBusy
			}
		}
		delete(d.PersistentResources, r.ID)
		return nil
	})
}

func validatePersistentResourceState(data environmentFileState) error {
	for id, r := range data.PersistentResources {
		if id != r.ID || !core.ValidPersistentResourceRef(r.Ref()) || r.Kind == "" || r.NativeRef == "" || r.CreatedAt.IsZero() || (r.State != "creating" && r.State != "ready" && r.State != "deleting") {
			return fmt.Errorf("invalid persistent resource catalog: %w", core.ErrIncompatibleState)
		}
	}

	for _, r := range data.PersistentResources {
		if r.CopySource == (core.PersistentResourceRef{}) {
			continue
		}
		source, ok := data.PersistentResources[r.CopySource.ID]
		if !ok || r.State != "creating" || source.State != "ready" || source.Ref() != r.CopySource || source.Kind != r.Kind || source.ID == r.ID || source.Owner == r.Owner || source.NativeRef == r.NativeRef {
			return fmt.Errorf("invalid persistent copy reservation: %w", core.ErrIncompatibleState)
		}
		for _, lease := range data.Leases {
			if lease.PersistentResource.ID == source.ID {
				return fmt.Errorf("copy source is attached: %w", core.ErrIncompatibleState)
			}
		}
	}
	held := map[string]string{}
	for environmentID, lease := range data.Leases {
		ref := lease.PersistentResource
		if ref == (core.PersistentResourceRef{}) {
			continue
		}
		resource, ok := data.PersistentResources[ref.ID]
		if !ok || resource.Ref() != ref || resource.State != "ready" {
			return fmt.Errorf("invalid persistent resource reservation: %w", core.ErrIncompatibleState)
		}
		if _, duplicate := held[ref.ID]; duplicate {
			return fmt.Errorf("duplicate persistent resource reservation: %w", core.ErrIncompatibleState)
		}
		held[ref.ID] = environmentID
	}
	for name, environment := range data.Environments {
		if environment.PersistentResource != (core.PersistentResourceRef{}) {
			lease, ok := data.Leases[name]
			if !ok || lease.PersistentResource != environment.PersistentResource {
				return fmt.Errorf("persistent attachment differs from lease: %w", core.ErrIncompatibleState)
			}
		}
	}
	return nil
}

// BeginPersistentResourceCopy atomically reserves the source and records the
// exact destination identity before the provider can start copying.
func (s *EnvironmentJSONStore) BeginPersistentResourceCopy(_ context.Context, source, target core.PersistentResource) error {
	if !core.ValidPersistentResourceRef(target.Ref()) || target.ID == source.ID || target.Owner == source.Owner || target.Kind != source.Kind || target.NativeRef == "" || target.NativeRef == source.NativeRef || target.State != "creating" || target.CreatedAt.IsZero() || target.CopySource != source.Ref() {
		return core.ErrInvalidArgument
	}
	return s.resourceTransaction(func(d *environmentFileState) error {
		if _, exists := d.PersistentResources[target.ID]; exists {
			return core.ErrAlreadyExists
		}
		if current, ok := d.PersistentResources[source.ID]; !ok || current != source || current.State != "ready" {
			return core.ErrIncompatibleState
		}
		if persistentCopyReserved(*d, source.ID) {
			return core.ErrStorageBusy
		}
		for _, lease := range d.Leases {
			if lease.PersistentResource.ID == source.ID {
				return core.ErrStorageBusy
			}
		}
		for _, env := range d.Environments {
			if env.PersistentResource.ID == source.ID {
				return core.ErrStorageBusy
			}
		}
		d.PersistentResources[target.ID] = target
		return nil
	})
}

func persistentCopyReserved(d environmentFileState, id string) bool {
	for _, r := range d.PersistentResources {
		if r.CopySource.ID == id {
			return true
		}
	}
	return false
}
