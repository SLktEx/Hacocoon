package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
)

type networkConnectionProvider interface {
	DialEnvironmentNetwork(context.Context, string, string, string, int) (net.Conn, error)
}

// DialEnvironmentNetwork resolves the persisted provider route before asking the
// selected adapter to pin a connection to its exact creation identity.
func (r *Router) DialEnvironmentNetwork(ctx context.Context, ref, instance, protocol string, port int) (net.Conn, error) {
	provider, native, err := r.resolve(ref)
	if err != nil {
		return nil, err
	}
	network, ok := provider.(networkConnectionProvider)
	if !ok {
		return nil, core.ErrUnsupported
	}
	return network.DialEnvironmentNetwork(ctx, native, instance, protocol, port)
}
