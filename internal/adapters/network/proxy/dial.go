package egressproxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (p *Proxy) resolvePinned(ctx context.Context, host string) ([]netip.Addr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolved, err := p.resolver.LookupIPAddr(ctx, host)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, &upstreamFailure{reason: "dns_lookup_failed", err: err}
	}
	if len(resolved) == 0 {
		return nil, &upstreamFailure{reason: "dns_empty_result", err: core.ErrRuntimeUnavailable}
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := map[netip.Addr]struct{}{}
	for _, item := range resolved {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok || item.Zone != "" {
			return nil, &upstreamFailure{reason: "address_disallowed", err: core.ErrPolicyDenied}
		}
		addr = addr.Unmap()
		if addr.IsLoopback() {
			return nil, &upstreamFailure{reason: "address_loopback", err: core.ErrPolicyDenied}
		}
		if !allowedDialAddress(addr) {
			// Reject the whole answer set, including when an allowed public/private
			// address precedes an unsafe answer. Never dial a partial result.
			return nil, &upstreamFailure{reason: "address_disallowed", err: core.ErrPolicyDenied}
		}
		if _, exists := seen[addr]; exists {
			continue
		}
		seen[addr] = struct{}{}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

func (p *Proxy) dialPinned(ctx context.Context, addresses []netip.Addr, port int) (net.Conn, error) {
	var errs []error
	for _, address := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := p.dial(ctx, "tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if canceled := ctx.Err(); canceled != nil {
			if conn != nil {
				_ = conn.Close()
			}
			return nil, canceled
		}
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil, &upstreamFailure{reason: "dial_failed", err: core.ErrRuntimeUnavailable}
	}
	return nil, &upstreamFailure{reason: "dial_failed", err: errors.Join(errs...)}
}

func allowedDialAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.Zone() != "" || !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range forbiddenDialPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var forbiddenDialPrefixes = mustPrefixes(
	// Go's global-unicast predicate includes these IPv4 reserved ranges.
	// Neither is an ordinary unicast destination. Private/shared unicast
	// networks are allowed after the separate hostname-scoped authorization.
	"0.0.0.0/8", "240.0.0.0/4",
)

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParsePrefix(value))
	}
	return result
}
