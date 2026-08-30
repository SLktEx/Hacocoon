package environment

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// EgressProxyProvider is an optional provider capability. Core owns the
// auditable hostname policy; runtime adapters own the mechanism that exposes a
// broker endpoint inside one concrete Environment.
type EgressProxyProvider interface {
	EnsureEgressProxy(context.Context, string, string) error
}

func (r *Router) EnsureEgressProxy(ctx context.Context, rawRef, socketPath string) error {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return err
	}
	egress, ok := provider.(EgressProxyProvider)
	if !ok {
		return fmt.Errorf("environment provider %q egress proxy: %w", id, core.ErrUnsupported)
	}
	return egress.EnsureEgressProxy(ctx, ref, socketPath)
}
