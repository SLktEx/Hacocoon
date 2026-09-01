package incus

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	sandboxRoutedHostIPv4       = "169.254.254.1"
	sandboxRoutedGuestPool      = "198.18.0.0/15"
	sandboxRoutedHostPrefix     = "haco"
	sandboxRoutedFirewallFamily = "inet"
	sandboxRoutedFirewallTable  = "hacocoon_sandbox"
)

var sandboxRoutedPool = netip.MustParsePrefix(sandboxRoutedGuestPool)

// ensureRoutedSandboxHost prepares the host-side substrate used by every
// Hacocoon Environment. Environments use point-to-point routed NICs rather than
// sharing an Ethernet bridge. This keeps the Linux and WSL networking model the
// same and avoids any dependency on nftables' bridge family.
func (r *Runtime) ensureRoutedSandboxHost(ctx context.Context) error {
	if r == nil || r.runner == nil {
		return core.ErrInvalidArgument
	}
	if err := r.ensureRoutedProxyAddress(ctx); err != nil {
		return err
	}
	if err := r.ensureRoutedPoolAvailable(ctx); err != nil {
		return err
	}
	return r.ensureRoutedSandboxFirewall(ctx)
}

func (r *Runtime) runRoutedPrivileged(ctx context.Context, command string, args ...string) (host.Result, error) {
	sudoArgs := []string{"-n", "--", command}
	sudoArgs = append(sudoArgs, args...)
	return r.runner.Run(ctx, "sudo", sudoArgs...)
}

func (r *Runtime) ensureRoutedProxyAddress(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show")
	if err != nil {
		return fmt.Errorf("inspect Hacocoon routed proxy address: %w", err)
	}
	loopbackFound := false
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		iface := strings.TrimSuffix(fields[1], ":")
		for i := 2; i+1 < len(fields); i++ {
			if fields[i] != "inet" {
				continue
			}
			prefix, parseErr := netip.ParsePrefix(fields[i+1])
			if parseErr != nil || prefix.Addr().String() != sandboxRoutedHostIPv4 {
				continue
			}
			switch {
			case iface == "lo":
				loopbackFound = true
			case strings.HasPrefix(iface, sandboxRoutedHostPrefix):
				// Incus' routed NIC assigns ipv4.host_address to the host-side
				// veth as well as using it as the guest gateway. The same /32 on
				// a Hacocoon-managed point-to-point interface is therefore
				// expected and does not widen the trusted host surface.
			default:
				return fmt.Errorf("Hacocoon routed proxy address %s is already owned by unmanaged interface %s: %w", sandboxRoutedHostIPv4, iface, core.ErrIncompatibleState)
			}
		}
	}
	if !loopbackFound {
		if _, addErr := r.runRoutedPrivileged(ctx, "ip", "address", "add", sandboxRoutedHostIPv4+"/32", "dev", "lo"); addErr != nil {
			// A concurrent Environment creator can win the add race. Re-read the
			// authoritative host state before treating the mutation as failed.
			verified, verifyErr := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show", "dev", "lo")
			if verifyErr != nil || !strings.Contains(verified.Stdout, sandboxRoutedHostIPv4+"/32") {
				return fmt.Errorf("install Hacocoon routed proxy address: %w", addErr)
			}
		}
	}
	verified, err := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show", "dev", "lo")
	if err != nil || !strings.Contains(verified.Stdout, sandboxRoutedHostIPv4+"/32") {
		if err != nil {
			return fmt.Errorf("verify Hacocoon routed proxy address: %w", err)
		}
		return fmt.Errorf("Hacocoon routed proxy address did not persist: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) ensureRoutedPoolAvailable(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "ip", "-4", "route", "show")
	if err != nil {
		return fmt.Errorf("inspect routes before reserving Hacocoon sandbox pool: %w", err)
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "default" {
			continue
		}
		var prefix netip.Prefix
		if strings.Contains(fields[0], "/") {
			parsed, parseErr := netip.ParsePrefix(fields[0])
			if parseErr != nil || !parsed.Addr().Is4() {
				continue
			}
			prefix = parsed
		} else {
			addr, parseErr := netip.ParseAddr(fields[0])
			if parseErr != nil || !addr.Is4() {
				continue
			}
			prefix = netip.PrefixFrom(addr, 32)
		}
		if !prefixesOverlap(prefix, sandboxRoutedPool) {
			continue
		}
		iface := routeDevice(fields)
		if strings.HasPrefix(iface, sandboxRoutedHostPrefix) && prefix.Bits() == 32 {
			continue
		}
		return fmt.Errorf("host route %q overlaps reserved Hacocoon sandbox pool %s: %w", strings.TrimSpace(line), sandboxRoutedGuestPool, core.ErrIncompatibleState)
	}
	return nil
}

func routeDevice(fields []string) string {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "dev" {
			return fields[i+1]
		}
	}
	return ""
}

func prefixesOverlap(a, b netip.Prefix) bool {
	return a.Contains(b.Addr()) || b.Contains(a.Addr())
}

func (r *Runtime) ensureRoutedSandboxFirewall(ctx context.Context) error {
	shown, err := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
	if err == nil {
		return verifyRoutedSandboxFirewall(shown.Stdout)
	}

	created := false
	rollback := func() {
		if created {
			_, _ = r.runRoutedPrivileged(context.WithoutCancel(ctx), "nft", "delete", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
		}
	}
	if _, err := r.runRoutedPrivileged(ctx, "nft", "add", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable); err != nil {
		// Another concurrent creator may have won the race. Verify rather than
		// weakening or replacing an existing ruleset.
		shown, showErr := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
		if showErr == nil {
			return verifyRoutedSandboxFirewall(shown.Stdout)
		}
		return fmt.Errorf("create Hacocoon routed firewall table: %w", err)
	}
	created = true

	commands := [][]string{
		{"add", "chain", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "{", "type", "filter", "hook", "input", "priority", "-200", ";", "policy", "accept", ";", "}"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "ip", "daddr", sandboxRoutedHostIPv4, "tcp", "dport", fmt.Sprintf("%d", sandboxEgressProxyPort), "accept"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
		{"add", "chain", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "{", "type", "filter", "hook", "forward", "priority", "-200", ";", "policy", "accept", ";", "}"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "oifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
	}
	for _, args := range commands {
		if _, err := r.runRoutedPrivileged(ctx, "nft", args...); err != nil {
			rollback()
			return fmt.Errorf("configure Hacocoon routed firewall: %w", err)
		}
	}
	shown, err = r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
	if err != nil {
		rollback()
		return fmt.Errorf("verify Hacocoon routed firewall: %w", err)
	}
	if err := verifyRoutedSandboxFirewall(shown.Stdout); err != nil {
		rollback()
		return err
	}
	return nil
}

func verifyRoutedSandboxFirewall(raw string) error {
	expectedRules := map[string][]string{
		"input": {
			"iifname \"" + sandboxRoutedHostPrefix + "*\" ip daddr " + sandboxRoutedHostIPv4 + " tcp dport " + fmt.Sprintf("%d", sandboxEgressProxyPort) + " accept",
			"iifname \"" + sandboxRoutedHostPrefix + "*\" drop",
		},
		"forward": {
			"iifname \"" + sandboxRoutedHostPrefix + "*\" drop",
			"oifname \"" + sandboxRoutedHostPrefix + "*\" drop",
		},
	}
	seenRules := map[string][]string{"input": {}, "forward": {}}
	seenHooks := map[string]bool{}
	chain := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "chain input ") || trimmed == "chain input {":
			chain = "input"
		case strings.HasPrefix(trimmed, "chain forward ") || trimmed == "chain forward {":
			chain = "forward"
		case strings.HasPrefix(trimmed, "chain "):
			return fmt.Errorf("Hacocoon routed firewall contains unmanaged chain %q: %w", trimmed, core.ErrIncompatibleState)
		case strings.HasPrefix(trimmed, "type filter hook "):
			if chain == "" || !strings.Contains(trimmed, "hook "+chain) || !strings.Contains(trimmed, "policy accept") {
				return fmt.Errorf("Hacocoon routed firewall chain %q has unsafe hook %q: %w", chain, trimmed, core.ErrIncompatibleState)
			}
			seenHooks[chain] = true
		case chain != "" && trimmed != "" && trimmed != "}" && !strings.HasPrefix(trimmed, "table "):
			seenRules[chain] = append(seenRules[chain], trimmed)
		}
	}
	for _, name := range []string{"input", "forward"} {
		if !seenHooks[name] {
			return fmt.Errorf("Hacocoon routed firewall is missing %s hook: %w", name, core.ErrIncompatibleState)
		}
		want := expectedRules[name]
		got := seenRules[name]
		if len(got) != len(want) {
			return fmt.Errorf("Hacocoon routed firewall %s rules = %q, want %q: %w", name, got, want, core.ErrIncompatibleState)
		}
		for i := range want {
			if got[i] != want[i] {
				return fmt.Errorf("Hacocoon routed firewall %s rule %d = %q, want %q: %w", name, i, got[i], want[i], core.ErrIncompatibleState)
			}
		}
	}
	return nil
}

func (r *Runtime) addRoutedSandboxNIC(ctx context.Context, ref string) error {
	if err := r.ensureRoutedSandboxHost(ctx); err != nil {
		return err
	}
	address, err := r.allocateRoutedSandboxIPv4(ctx, ref)
	if err != nil {
		return err
	}
	iface := routedSandboxHostInterface(ref)
	args := []string{
		"config", "device", "add", ref, "eth0", "nic",
		"name=eth0",
		"nictype=routed",
		"host_name=" + iface,
		"ipv4.address=" + address,
		"ipv4.host_address=" + sandboxRoutedHostIPv4,
		"ipv4.gateway=auto",
		"ipv4.neighbor_probe=false",
		"ipv6.gateway=none",
		"--project", r.project,
	}
	result, err := r.runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return fmt.Errorf("add routed sandbox NIC: %s: %w", reason, err)
		}
		return fmt.Errorf("add routed sandbox NIC: %w", err)
	}
	return nil
}

func (r *Runtime) verifyRoutedSandboxAntiSpoof(ctx context.Context, ref string) error {
	iface := routedSandboxHostInterface(ref)
	rpFilter, err := r.runner.Run(ctx, "cat", "/proc/sys/net/ipv4/conf/"+iface+"/rp_filter")
	if err != nil {
		return fmt.Errorf("verify routed sandbox rp_filter on %s: %w", iface, err)
	}
	if strings.TrimSpace(rpFilter.Stdout) != "1" {
		return fmt.Errorf("routed sandbox interface %s has rp_filter=%q, want strict mode 1: %w", iface, strings.TrimSpace(rpFilter.Stdout), core.ErrIncompatibleState)
	}

	address, err := r.runner.Run(ctx, "incus", "config", "device", "get", ref, "eth0", "ipv4.address", "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify routed sandbox IPv4 identity: %w", err)
	}
	parsed, parseErr := netip.ParseAddr(strings.TrimSpace(address.Stdout))
	if parseErr != nil || !sandboxRoutedPool.Contains(parsed) {
		return fmt.Errorf("routed sandbox IPv4 address %q is outside managed pool %s: %w", strings.TrimSpace(address.Stdout), sandboxRoutedGuestPool, core.ErrIncompatibleState)
	}
	routes, err := r.runner.Run(ctx, "ip", "-4", "route", "show", "dev", iface)
	if err != nil {
		return fmt.Errorf("verify routed sandbox host route: %w", err)
	}
	if !hasExactRoutedSandboxHostRoute(routes.Stdout, parsed) {
		return fmt.Errorf("routed sandbox address %s has no exact host route on %s (routes=%q): %w", parsed, iface, strings.TrimSpace(routes.Stdout), core.ErrIncompatibleState)
	}
	return nil
}

func hasExactRoutedSandboxHostRoute(raw string, address netip.Addr) bool {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		destination := fields[0]
		if destination == address.String() || destination == address.String()+"/32" {
			return true
		}
	}
	return false
}

func routedSandboxHostInterface(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	value := binary.BigEndian.Uint64(sum[:8]) & 0xffffffffff
	return fmt.Sprintf("%s%010x", sandboxRoutedHostPrefix, value)
}

func (r *Runtime) allocateRoutedSandboxIPv4(ctx context.Context, ref string) (string, error) {
	result, err := r.runner.Run(ctx, "incus", "list", "--project", r.project, "--format", "json")
	if err != nil {
		return "", fmt.Errorf("inspect routed sandbox address allocations: %w", err)
	}
	var instances []struct {
		Name            string                       `json:"name"`
		Devices         map[string]map[string]string `json:"devices"`
		ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &instances); err != nil {
		return "", fmt.Errorf("decode routed sandbox address allocations: %w", err)
	}
	used := map[string]struct{}{}
	for _, instance := range instances {
		if instance.Name == ref {
			continue
		}
		device := instance.Devices["eth0"]
		if len(device) == 0 {
			device = instance.ExpandedDevices["eth0"]
		}
		if device["nictype"] != "routed" {
			continue
		}
		if address := strings.TrimSpace(device["ipv4.address"]); address != "" {
			used[address] = struct{}{}
		}
	}

	sum := sha256.Sum256([]byte(ref))
	start := binary.BigEndian.Uint32(sum[:4]) % 131072
	for probe := uint32(0); probe < 131072; probe++ {
		candidate := routedSandboxIPv4At((start + probe) % 131072)
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Hacocoon routed sandbox address pool %s is exhausted: %w", sandboxRoutedGuestPool, core.ErrIncompatibleState)
}

func routedSandboxIPv4At(offset uint32) string {
	second := byte(18 + ((offset >> 16) & 0x1))
	third := byte((offset >> 8) & 0xff)
	fourth := byte(offset & 0xff)
	return netip.AddrFrom4([4]byte{198, second, third, fourth}).String()
}
