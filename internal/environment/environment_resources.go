package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Router) SupportsEnvironmentResources() bool {
	if r == nil {
		return false
	}
	provider, err := r.provider(r.defaultProvider)
	if err != nil {
		return false
	}
	support, ok := provider.(interface{ SupportsEnvironmentResources() bool })
	return ok && support.SupportsEnvironmentResources()
}

func (r *Router) StartEnvironmentWithResources(ctx context.Context, rawRef, instance string, areas []core.EnvironmentRuntimeAttachment) error {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return err
	}
	starter, ok := provider.(interface {
		StartEnvironmentWithResources(context.Context, string, string, []core.EnvironmentRuntimeAttachment) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	return starter.StartEnvironmentWithResources(ctx, ref, instance, areas)
}
