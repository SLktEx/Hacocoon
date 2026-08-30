package incus

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/egressproxy"
)

const sandboxEgressProxyPort = egressproxy.DefaultPort

func (r *Runtime) sandboxGateway(ctx context.Context) (netip.Addr, error) {
	if r == nil || r.runner == nil {
		return netip.Addr{}, core.ErrInvalidArgument
	}
	result, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "ipv4.address", "--project", sandboxResourceProject)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("read Hacocoon sandbox gateway: %w", err)
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(result.Stdout))
	if err != nil || !prefix.Addr().Is4() || prefix.Addr().IsUnspecified() {
		return netip.Addr{}, fmt.Errorf("invalid Hacocoon sandbox IPv4 address %q: %w", strings.TrimSpace(result.Stdout), core.ErrIncompatibleState)
	}
	return prefix.Addr(), nil
}

func (r *Runtime) sandboxProxyURL(ctx context.Context) (string, error) {
	gateway, err := r.sandboxGateway(ctx)
	if err != nil {
		return "", err
	}
	// Keep the URI path explicit. Ubuntu 26.04 APT can otherwise treat a bare
	// numeric proxy authority such as 10.0.0.1:18080 as a hostname during
	// Acquire setup when DNS is deliberately disabled inside the sandbox.
	return "http://" + net.JoinHostPort(gateway.String(), strconv.Itoa(sandboxEgressProxyPort)) + "/", nil
}

// PrepareEgressProxy verifies the managed fail-closed network substrate and
// returns the only address on which the Standard egress proxy is expected to
// listen. The address is derived from the Hacocoon-owned bridge, not from a
// caller-supplied host interface.
func (r *Runtime) PrepareEgressProxy(ctx context.Context) (string, error) {
	if err := r.ensureSandboxNetwork(ctx); err != nil {
		return "", fmt.Errorf("ensure Hacocoon egress network: %w", err)
	}
	gateway, err := r.sandboxGateway(ctx)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(gateway.String(), strconv.Itoa(sandboxEgressProxyPort)), nil
}

// ResolveEnvironment maps a proxy connection's source IP back to exactly one
// Hacocoon-managed Incus instance. security.ipv4_filtering on the sandbox NIC
// prevents the guest from choosing an arbitrary source address, and the
// Environment identity is never accepted from proxy request metadata.
func (r *Runtime) ResolveEnvironment(ctx context.Context, source net.IP) (string, error) {
	if r == nil || r.runner == nil || source == nil || source.IsUnspecified() || source.IsLoopback() || source.IsMulticast() {
		return "", core.ErrPolicyDenied
	}
	ip := source.String()
	if parsed := net.ParseIP(ip); parsed == nil {
		return "", core.ErrPolicyDenied
	}
	result, err := r.runner.Run(ctx, "incus", "list", "ipv4="+ip, "--project", r.project, "--format", "csv", "-c", "n")
	if err != nil {
		return "", fmt.Errorf("resolve egress source %s: %w", ip, err)
	}
	var refs []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		ref := strings.TrimSpace(line)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	if len(refs) != 1 || !strings.HasPrefix(refs[0], "haco-") || len(refs[0]) == len("haco-") {
		return "", core.ErrPolicyDenied
	}
	return strings.TrimPrefix(refs[0], "haco-"), nil
}
