package environment

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type BaseCatalogProvider interface {
	ListBases(context.Context) ([]core.BaseInfo, error)
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
}

// BaseRouter preserves the v0.7 provider routing contract while forwarding
// provider-neutral creation metadata (Base identity and ResourceBudget).
type BaseRouter struct {
	*Router
}

func NewBaseRouter(router *Router) *BaseRouter {
	return &BaseRouter{Router: router}
}

func (r *BaseRouter) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if r == nil || r.Router == nil {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	provider, err := r.provider(r.defaultProvider)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	created, err := provider.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if strings.TrimSpace(created.Ref) == "" {
		return core.EnvironmentRuntime{}, fmt.Errorf("provider %q returned empty runtime ref: %w", r.defaultProvider, core.ErrIncompatibleState)
	}
	return core.EnvironmentRuntime{
		Ref:       encodeRouteRef(r.defaultProvider, created.Ref),
		Base:      created.Base,
		Resources: created.Resources,
	}, nil
}

func (r *BaseRouter) ListBases(ctx context.Context) ([]core.BaseInfo, error) {
	if r == nil || r.Router == nil {
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

func (r *BaseRouter) InspectBase(ctx context.Context, name core.BaseName) (core.BaseInfo, error) {
	if r == nil || r.Router == nil {
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
