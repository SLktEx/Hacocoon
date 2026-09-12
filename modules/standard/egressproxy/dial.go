package egressproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (p *Proxy) resolvePinned(ctx context.Context, host string) ([]netip.Addr, error) {
	resolved, err := p.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := map[netip.Addr]struct{}{}
	for _, item := range resolved {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return nil, core.ErrPolicyDenied
		}
		addr = addr.Unmap()
		if !publicDialAddress(addr) {
			// Reject the whole answer set. Silently dropping a private answer would
			// make mixed/rebinding responses dependent on resolver ordering.
			return nil, core.ErrPolicyDenied
		}
		if _, exists := seen[addr]; exists {
			continue
		}
		seen[addr] = struct{}{}
		addresses = append(addresses, addr)
	}
	if len(addresses) == 0 {
		return nil, core.ErrPolicyDenied
	}
	return addresses, nil
}

func (p *Proxy) dialPinned(ctx context.Context, addresses []netip.Addr, port int) (net.Conn, error) {
	var errs []error
	for _, address := range addresses {
		conn, err := p.dial(ctx, "tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil, core.ErrRuntimeUnavailable
	}
	return nil, errors.Join(errs...)
}

func publicDialAddress(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
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
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	"2001:db8::/32",
)

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParsePrefix(value))
	}
	return result
}
