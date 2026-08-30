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
	return "http://" + net.JoinHostPort(gateway.String(), strconv.Itoa(sandboxEgressProxyPort)), nil
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

// ResolveSourceIP maps a proxy connection's source IP to exactly one
// provider-local Incus runtime reference. This method deliberately does not
// assign Hacocoon Environment authority; the Environment adapter must bind this
// runtime ref to persisted managed Environment state before policy evaluation.
func (r *Runtime) ResolveSourceIP(ctx context.Context, source net.IP) (string, error) {
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
		if ref == "" {
			continue
		}
		if strings.ContainsAny(ref, "\r\n\x00") {
			return "", fmt.Errorf("Incus source lookup returned an invalid runtime ref: %w", core.ErrIncompatibleState)
		}
		refs = append(refs, ref)
	}
	switch len(refs) {
	case 0:
		return "", core.ErrNotFound
	case 1:
		return refs[0], nil
	default:
		return "", fmt.Errorf("egress source %s maps to multiple Incus runtimes: %w", ip, core.ErrIncompatibleState)
	}
}
