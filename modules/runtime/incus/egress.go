package incus

import (
	"context"
	"encoding/json"
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
	if err := r.ensureRoutedSandboxHost(ctx); err != nil {
		return netip.Addr{}, err
	}
	address, err := netip.ParseAddr(sandboxRoutedHostIPv4)
	if err != nil || !address.Is4() || address.IsUnspecified() {
		return netip.Addr{}, fmt.Errorf("invalid Hacocoon routed host IPv4 address %q: %w", sandboxRoutedHostIPv4, core.ErrIncompatibleState)
	}
	return address, nil
}

func (r *Runtime) sandboxProxyURL(ctx context.Context) (string, error) {
	gateway, err := r.sandboxGateway(ctx)
	if err != nil {
		return "", err
	}
	return "http://" + net.JoinHostPort(gateway.String(), strconv.Itoa(sandboxEgressProxyPort)), nil
}

// PrepareEgressProxy verifies the managed fail-closed routed substrate and
// returns the only address on which the Standard egress proxy is expected to
// listen. The address is Hacocoon-owned loopback state reachable from each
// point-to-point routed NIC, rather than a shared Ethernet bridge.
func (r *Runtime) PrepareEgressProxy(ctx context.Context) (string, error) {
	if err := r.ensureRoutedSandboxHost(ctx); err != nil {
		return "", fmt.Errorf("ensure Hacocoon routed egress network: %w", err)
	}
	gateway, err := r.sandboxGateway(ctx)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(gateway.String(), strconv.Itoa(sandboxEgressProxyPort)), nil
}

// ResolveRuntimeRef maps a proxy connection's source IP back to exactly one
// Hacocoon-managed Incus runtime ref. Routed NICs remove the shared-L2 attack
// surface, while the verified strict host-side rp_filter prevents a guest from
// using a source address that is not routed to its own point-to-point veth.
// Persisted Environment identity is intentionally resolved outside the Incus
// provider.
func (r *Runtime) ResolveRuntimeRef(ctx context.Context, source net.IP) (string, error) {
	if r == nil || r.runner == nil || source == nil || source.IsUnspecified() || source.IsLoopback() || source.IsMulticast() {
		return "", core.ErrPolicyDenied
	}
	ip := source.String()
	if parsed := net.ParseIP(ip); parsed == nil {
		return "", core.ErrPolicyDenied
	}

	// Prefer Incus' native address shorthand because it avoids fetching state
	// for every instance on the hot egress path. Incus 7 can transiently return
	// an empty shorthand-filter result while a freshly-started instance already
	// has its static routed address in runtime state, so zero matches fall back
	// to the authoritative JSON state below. Ambiguous results always fail
	// closed.
	result, err := r.runner.Run(ctx, "incus", "list", "ipv4="+ip, "--project", r.project, "--format", "csv", "-c", "n")
	if err != nil {
		return "", fmt.Errorf("resolve egress source %s: %w", ip, err)
	}
	refs := parseRuntimeRefs(result.Stdout)
	if len(refs) == 1 {
		if validateManagedInstanceRef(refs[0]) != nil {
			return "", core.ErrPolicyDenied
		}
		return refs[0], nil
	}
	if len(refs) > 1 {
		return "", core.ErrPolicyDenied
	}

	full, err := r.runner.Run(ctx, "incus", "list", "--project", r.project, "--format", "json")
	if err != nil {
		return "", fmt.Errorf("resolve egress source %s from Incus runtime state: %w", ip, err)
	}
	refs, err = runtimeRefsForIPv4(full.Stdout, ip)
	if err != nil {
		return "", fmt.Errorf("decode Incus runtime state for egress source %s: %w", ip, err)
	}
	if len(refs) != 1 || validateManagedInstanceRef(refs[0]) != nil {
		return "", core.ErrPolicyDenied
	}
	return refs[0], nil
}

func parseRuntimeRefs(raw string) []string {
	var refs []string
	for _, line := range strings.Split(raw, "\n") {
		ref := strings.TrimSpace(line)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func runtimeRefsForIPv4(raw, ip string) ([]string, error) {
	var instances []struct {
		Name  string `json:"name"`
		State struct {
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}
	if err := json.Unmarshal([]byte(raw), &instances); err != nil {
		return nil, err
	}

	var refs []string
	for _, instance := range instances {
		matched := false
		for _, iface := range instance.State.Network {
			for _, address := range iface.Addresses {
				if address.Family == "inet" && address.Address == ip {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			refs = append(refs, instance.Name)
		}
	}
	return refs, nil
}

// ResolveEnvironment is kept for compatibility with existing direct provider
// tests/callers. New egress authorization wiring should bind ResolveRuntimeRef
// to persisted Hacocoon Environment state before returning an identity.
func (r *Runtime) ResolveEnvironment(ctx context.Context, source net.IP) (string, error) {
	ref, err := r.ResolveRuntimeRef(ctx, source)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(ref, "haco-"), nil
}
