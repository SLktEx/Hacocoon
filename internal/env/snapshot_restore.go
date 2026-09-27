package environment

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type legacyRestoreCleanupProvider interface {
	DeleteRestoreComponent(context.Context, core.SnapshotComponent) error
}

func (r *Router) nativeSavedSnapshot(s core.Snapshot, expected string) (core.Snapshot, error) {
	_, native, id, err := r.resolveWithID(s.Source.Environment.RuntimeRef)
	if err != nil {
		return s, err
	}
	if id != expected {
		return s, core.ErrCapabilityStale
	}
	s.Source.Environment.RuntimeRef = native
	s.Components = append([]core.SnapshotComponent(nil), s.Components...)
	for i, c := range s.Components {
		_, decoded, provider, err := r.snapshotBackend(c)
		if err != nil {
			return s, err
		}
		if provider != expected {
			return s, core.ErrCapabilityStale
		}
		s.Components[i] = decoded
	}
	return s, nil
}
func (r *Router) restoreBackend(c core.SnapshotComponent) (legacyRestoreCleanupProvider, core.SnapshotComponent, string, error) {
	if !strings.HasPrefix(c.NativeRef, refPrefix) {
		return nil, c, "", core.ErrIncompatibleState
	}
	p, native, id, err := r.resolveWithID(c.NativeRef)
	if err != nil {
		return nil, c, "", err
	}
	if c.NativeRef != encodeRouteRef(id, native) {
		return nil, c, "", core.ErrIncompatibleState
	}
	backend, ok := p.(legacyRestoreCleanupProvider)
	if !ok {
		return nil, c, "", core.ErrUnsupported
	}
	c.NativeRef = native
	return backend, c, id, nil
}
func (r *Router) DeleteRestoreComponent(ctx context.Context, c core.SnapshotComponent) error {
	backend, c, _, err := r.restoreBackend(c)
	if err != nil {
		return err
	}
	return backend.DeleteRestoreComponent(ctx, c)
}
