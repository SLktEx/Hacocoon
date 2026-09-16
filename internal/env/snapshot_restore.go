package environment

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type restoreProvider interface {
	PlanSnapshotRestore(context.Context, core.Snapshot, string) ([]core.SnapshotComponent, error)
	CreateRestoreComponent(context.Context, core.Snapshot, core.SnapshotComponent) error
	VerifyRestoreComponent(context.Context, core.SnapshotComponent) error
	DeleteRestoreComponent(context.Context, core.SnapshotComponent) error
}

func (r *Router) nativeRestoreSnapshot(s core.Snapshot, expected string) (core.Snapshot, error) {
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
func (r *Router) PlanSnapshotRestore(ctx context.Context, s core.Snapshot, id string) ([]core.SnapshotComponent, error) {
	p, _, provider, err := r.resolveWithID(s.Source.Environment.RuntimeRef)
	if err != nil {
		return nil, err
	}
	backend, ok := p.(restoreProvider)
	if !ok {
		return nil, core.ErrUnsupported
	}
	s, err = r.nativeRestoreSnapshot(s, provider)
	if err != nil {
		return nil, err
	}
	cs, err := backend.PlanSnapshotRestore(ctx, s, id)
	if err != nil {
		return nil, err
	}
	out := append([]core.SnapshotComponent(nil), cs...)
	for i, c := range out {
		if c.NativeRef == "" || strings.HasPrefix(c.NativeRef, refPrefix) {
			return nil, core.ErrIncompatibleState
		}
		out[i].NativeRef = encodeRouteRef(provider, c.NativeRef)
	}
	return out, nil
}
func (r *Router) restoreBackend(c core.SnapshotComponent) (restoreProvider, core.SnapshotComponent, string, error) {
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
	backend, ok := p.(restoreProvider)
	if !ok {
		return nil, c, "", core.ErrUnsupported
	}
	c.NativeRef = native
	return backend, c, id, nil
}
func (r *Router) CreateRestoreComponent(ctx context.Context, s core.Snapshot, c core.SnapshotComponent) error {
	backend, c, id, err := r.restoreBackend(c)
	if err != nil {
		return err
	}
	s, err = r.nativeRestoreSnapshot(s, id)
	if err != nil {
		return err
	}
	return backend.CreateRestoreComponent(ctx, s, c)
}
func (r *Router) VerifyRestoreComponent(ctx context.Context, c core.SnapshotComponent) error {
	backend, c, _, err := r.restoreBackend(c)
	if err != nil {
		return err
	}
	return backend.VerifyRestoreComponent(ctx, c)
}
func (r *Router) DeleteRestoreComponent(ctx context.Context, c core.SnapshotComponent) error {
	backend, c, _, err := r.restoreBackend(c)
	if err != nil {
		return err
	}
	return backend.DeleteRestoreComponent(ctx, c)
}
