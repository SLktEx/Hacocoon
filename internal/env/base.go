package environment

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type BaseCatalogProvider interface {
	ListBases(context.Context) ([]core.BaseInfo, error)
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
}

func (r *Router) ListBases(ctx context.Context) ([]core.BaseInfo, error) {
	if r == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	provider, err := r.provider(r.defaultProvider)
	if err != nil {
		return nil, err
	}
	catalog, ok := provider.(BaseCatalogProvider)
	if !ok {
		return nil, fmt.Errorf("environment provider %q Base catalog: %w", r.defaultProvider, core.ErrUnsupported)
	}
	return catalog.ListBases(ctx)
}

func (r *Router) InspectBase(ctx context.Context, name core.BaseName) (core.BaseInfo, error) {
	if r == nil {
		return core.BaseInfo{}, core.ErrRuntimeUnavailable
	}
	provider, err := r.provider(r.defaultProvider)
	if err != nil {
		return core.BaseInfo{}, err
	}
	catalog, ok := provider.(BaseCatalogProvider)
	if !ok {
		return core.BaseInfo{}, fmt.Errorf("environment provider %q Base catalog: %w", r.defaultProvider, core.ErrUnsupported)
	}
	return catalog.InspectBase(ctx, name)
}

func (r *Router) PublishBase(ctx context.Context, env core.Environment, lease core.WorkspaceLease, name core.BaseName) (core.BaseInfo, error) {
	// Check the persisted ownership pair before unwrapping either route. Replacing
	// a mismatched lease ref would hide the mismatch from the provider's checks.
	if lease.RuntimeRef != env.RuntimeRef {
		return core.BaseInfo{}, core.ErrInvalidArgument
	}
	provider, native, err := r.resolve(env.RuntimeRef)
	if err != nil {
		return core.BaseInfo{}, err
	}
	publisher, ok := provider.(interface {
		PublishBase(context.Context, core.Environment, core.WorkspaceLease, core.BaseName) (core.BaseInfo, error)
	})
	if !ok {
		return core.BaseInfo{}, core.ErrUnsupported
	}
	env.RuntimeRef = native
	lease.RuntimeRef = native
	return publisher.PublishBase(ctx, env, lease, name)
}
