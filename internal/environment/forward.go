package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
)

func (r *Router) DialEnvironmentTCP(ctx context.Context, ref, instance, address string, port int) (net.Conn, error) {
	if !core.ValidForwardAddress(address, port) || !core.ValidEnvironmentInstanceID(instance) {
		return nil, core.ErrInvalidArgument
	}
	provider, native, err := r.resolve(ref)
	if err != nil {
		return nil, err
	}
	tcp, ok := provider.(interface {
		DialEnvironmentTCP(context.Context, string, string, string, int) (net.Conn, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	return tcp.DialEnvironmentTCP(ctx, native, instance, address, port)
}
