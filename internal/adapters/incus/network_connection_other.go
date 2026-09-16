//go:build !linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"net/netip"
)

func (*Runtime) ExternalNetworkAddress(context.Context, netip.Addr) error { return core.ErrUnsupported }
func (*Runtime) DialEnvironmentNetwork(context.Context, string, string, string, int) (net.Conn, error) {
	return nil, core.ErrUnsupported
}

func (*Runtime) DialEnvironmentTCP(context.Context, string, string, string, int) (net.Conn, error) {
	return nil, core.ErrUnsupported
}

func (*Runtime) HostNetworkAddress(context.Context, netip.Addr) error { return core.ErrUnsupported }
func (*Runtime) DialDevelopmentNetwork(context.Context, netip.Addr, string, int, bool) (net.Conn, error) {
	return nil, core.ErrUnsupported
}
