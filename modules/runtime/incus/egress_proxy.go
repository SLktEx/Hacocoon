package incus

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

// PrepareEgressProxy verifies the managed Incus transport substrate and returns
// the bridge-local address on which the Standard proxy must listen. Binding the
// proxy to this address avoids exposing it on unrelated Host interfaces.
func (r *Runtime) PrepareEgressProxy(ctx context.Context) (string, error) {
	if r == nil || r.runner == nil {
		return "", core.ErrInvalidArgument
	}
	if err := r.ensureProject(ctx); err != nil {
		return "", fmt.Errorf("ensure Incus project for egress proxy: %w", err)
	}
	if err := r.ensureSandboxNetwork(ctx); err != nil {
		return "", fmt.Errorf("ensure Hacocoon proxy-only network: %w", err)
	}
	result, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "ipv4.address", "--project", sandboxResourceProject)
	if err != nil {
		return "", fmt.Errorf("resolve Hacocoon bridge gateway: %w", err)
	}
	gateway, err := parseSandboxGateway(result.Stdout)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(gateway.String(), strconv.Itoa(egressapp.DefaultProxyPort)), nil
}

func parseSandboxGateway(raw string) (netip.Addr, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil || !prefix.Addr().Is4() || prefix.Addr().IsUnspecified() || prefix.Addr().IsLoopback() {
		return netip.Addr{}, fmt.Errorf("invalid Hacocoon bridge IPv4 gateway %q: %w", strings.TrimSpace(raw), core.ErrIncompatibleState)
	}
	return prefix.Addr(), nil
}
