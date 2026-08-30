package control

import (
	"context"
	"net"
)

type peerIPContextKey struct{}

func withPeerIP(ctx context.Context, addr net.Addr) context.Context {
	if ctx == nil || addr == nil {
		return ctx
	}
	var ip net.IP
	switch value := addr.(type) {
	case *net.TCPAddr:
		ip = value.IP
	case *net.UDPAddr:
		ip = value.IP
	}
	if ip == nil {
		return ctx
	}
	return context.WithValue(ctx, peerIPContextKey{}, append(net.IP(nil), ip...))
}

// PeerIP returns the transport-authenticated TCP peer IP recorded by Server.
// Unix-domain clients intentionally have no peer IP. Callers must still bind
// this provider evidence to persisted Hacocoon state before granting authority.
func PeerIP(ctx context.Context) net.IP {
	if ctx == nil {
		return nil
	}
	ip, _ := ctx.Value(peerIPContextKey{}).(net.IP)
	return append(net.IP(nil), ip...)
}
