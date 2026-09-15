package environment

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type snapshotProvider interface {
	PlanSnapshot(context.Context, core.SnapshotSource, string) ([]core.SnapshotComponent, error)
	CreateSnapshotComponent(context.Context, core.SnapshotSource, core.SnapshotComponent) error
	VerifySnapshotComponent(context.Context, core.SnapshotComponent) error
	DeleteSnapshotComponent(context.Context, core.SnapshotComponent) error
}

func (r *Router) PlanSnapshot(ctx context.Context, s core.SnapshotSource, id string) ([]core.SnapshotComponent, error) {
	p, native, providerID, err := r.resolveWithID(s.Environment.RuntimeRef)
	if err != nil {
		return nil, err
	}
	backend, ok := p.(snapshotProvider)
	if !ok {
		return nil, core.ErrUnsupported
	}
	s.Environment.RuntimeRef = native
	planned, err := backend.PlanSnapshot(ctx, s, id)
	if err != nil {
		return nil, err
	}
	result := append([]core.SnapshotComponent(nil), planned...)
	for i, c := range result {
		if c.NativeRef == "" || strings.HasPrefix(c.NativeRef, refPrefix) {
			return nil, core.ErrIncompatibleState
		}
		result[i].NativeRef = encodeRouteRef(providerID, c.NativeRef)
	}
	return result, nil
}
func (r *Router) snapshotBackend(c core.SnapshotComponent) (snapshotProvider, core.SnapshotComponent, string, error) {
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
	backend, ok := p.(snapshotProvider)
	if !ok {
		return nil, c, "", core.ErrUnsupported
	}
	c.NativeRef = native
	return backend, c, id, nil
}
func (r *Router) CreateSnapshotComponent(ctx context.Context, s core.SnapshotSource, c core.SnapshotComponent) error {
	backend, c, id, err := r.snapshotBackend(c)
	if err != nil {
		return err
	}
	_, native, sourceID, err := r.resolveWithID(s.Environment.RuntimeRef)
	if err != nil {
		return err
	}
	if sourceID != id {
		return core.ErrCapabilityStale
	}
	s.Environment.RuntimeRef = native
	// No fallible work after create: the caller owns durable receipt ordering.
	return backend.CreateSnapshotComponent(ctx, s, c)
}
func (r *Router) VerifySnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	backend, c, _, err := r.snapshotBackend(c)
	if err != nil {
		return err
	}
	return backend.VerifySnapshotComponent(ctx, c)
}
func (r *Router) DeleteSnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	backend, c, _, err := r.snapshotBackend(c)
	if err != nil {
		return err
	}
	return backend.DeleteSnapshotComponent(ctx, c)
}
