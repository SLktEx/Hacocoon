package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/netip"
)

func (r *Router) ResolveEnvironmentName(ctx context.Context, rawRef, instance, name string) ([]netip.Addr, error) {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return nil, err
	}
	resolver, ok := provider.(core.BackendNameResolver)
	if !ok {
		return nil, core.ErrUnsupported
	}
	return resolver.ResolveEnvironmentName(ctx, ref, instance, name)
}
