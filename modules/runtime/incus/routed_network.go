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
	// The Standard egress proxy keeps one stable host-owned address while each
	// Environment receives a dedicated managed bridge. Guests reach this address
	// through their bridge gateway, but forwarding anywhere else is denied.
	sandboxRoutedProxyIPv4      = "169.254.254.1"
	sandboxRoutedGatewayIPv4    = "169.254.254.254" // compatibility only
	sandboxRoutedHostIPv4       = sandboxRoutedProxyIPv4
	sandboxRoutedGuestPool      = "198.18.0.0/15" // compatibility/test helper pool
	sandboxRoutedHostPrefix     = "hbr"
	sandboxRoutedFirewallFamily = "inet"
	sandboxRoutedFirewallTable  = "hacocoon_sandbox"
	sandboxRoutedGuardPrefix    = "haco_guard_"
	sandboxBridgeResourceProject = "default"
)

var sandboxRoutedPool = netip.MustParsePrefix(sandboxRoutedGuestPool)

// ensureRoutedSandboxHost prepares host state shared by all Environment
// bridges. The name is retained while the migration lands, but the data plane
// is deliberately bridged: every Environment gets its own L2 segment on both
// native Linux and WSL.
func (r *Runtime) ensureRoutedSandboxHost(ctx context.Context) error {
	if r == nil || r.runner == nil {
		return core.ErrInvalidArgument
	}
	if err := r.ensureRoutedProxyAddress(ctx); err != nil {
		return err
	}
	return r.ensureRoutedSandboxFirewall(ctx)
}

func (r *Runtime) runRoutedPrivileged(ctx context.Context, command string, args ...string) (host.Result, error) {
	sudoArgs := []string{"-n", "--", command}
	sudoArgs = append(sudoArgs, args...)
	return r.runner.Run(ctx, "sudo", sudoArgs...)
}

func nftTableMissing(result host.Result) bool {
	reason := strings.ToLower(result.Stderr)
	return strings.Contains(reason, "no such file") || strings.Contains(reason, "not found")
}

func (r *Runtime) ensureRoutedProxyAddress(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show")
	if err != nil {
		return fmt.Errorf("inspect Hacocoon proxy address: %w", err)
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
			if parseErr != nil || prefix.Addr().String() != sandboxRoutedProxyIPv4 {
				continue
			}
			if iface == "lo" {
				loopbackFound = true
				continue
			}
			return fmt.Errorf("Hacocoon proxy address %s is already owned by unmanaged interface %s: %w", sandboxRoutedProxyIPv4, iface, core.ErrIncompatibleState)
		}
	}
	if !loopbackFound {
		if _, addErr := r.runRoutedPrivileged(ctx, "ip", "address", "add", sandboxRoutedProxyIPv4+"/32", "dev", "lo"); addErr != nil {
			verified, verifyErr := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show", "dev", "lo")
			if verifyErr != nil || !strings.Contains(verified.Stdout, sandboxRoutedProxyIPv4+"/32") {
				return fmt.Errorf("install Hacocoon proxy address: %w", addErr)
			}
		}
	}
	verified, err := r.runner.Run(ctx, "ip", "-o", "-4", "address", "show", "dev", "lo")
	if err != nil || !strings.Contains(verified.Stdout, sandboxRoutedProxyIPv4+"/32") {
		if err != nil {
			return fmt.Errorf("verify Hacocoon proxy address: %w", err)
		}
		return fmt.Errorf("Hacocoon proxy address did not persist: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) ensureRoutedPoolAvailable(ctx context.Context) error {
	// Retained for callers/tests from the routed migration. Dedicated managed
	// bridges use Incus-selected non-overlapping subnets instead of this pool.
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
		shown, showErr := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
		if showErr == nil {
			return verifyRoutedSandboxFirewall(shown.Stdout)
		}
		return fmt.Errorf("create Hacocoon sandbox firewall table: %w", err)
	}
	created = true

	commands := [][]string{
		{"add", "chain", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "{", "type", "filter", "hook", "input", "priority", "-200", ";", "policy", "accept", ";", "}"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "ip", "daddr", sandboxRoutedProxyIPv4, "tcp", "dport", fmt.Sprintf("%d", sandboxEgressProxyPort), "accept"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "input", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
		{"add", "chain", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "{", "type", "filter", "hook", "forward", "priority", "-200", ";", "policy", "accept", ";", "}"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "iifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
		{"add", "rule", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable, "forward", "oifname", "\"" + sandboxRoutedHostPrefix + "*\"", "drop"},
	}
	for _, args := range commands {
		if _, err := r.runRoutedPrivileged(ctx, "nft", args...); err != nil {
			rollback()
			return fmt.Errorf("configure Hacocoon sandbox firewall: %w", err)
		}
	}
	shown, err = r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, sandboxRoutedFirewallTable)
	if err != nil {
		rollback()
		return fmt.Errorf("verify Hacocoon sandbox firewall: %w", err)
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
			"iifname \"" + sandboxRoutedHostPrefix + "*\" ip daddr " + sandboxRoutedProxyIPv4 + " tcp dport " + fmt.Sprintf("%d", sandboxEgressProxyPort) + " accept",
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
			return fmt.Errorf("Hacocoon sandbox firewall contains unmanaged chain %q: %w", trimmed, core.ErrIncompatibleState)
		case strings.HasPrefix(trimmed, "type filter hook "):
			if chain == "" || !strings.Contains(trimmed, "hook "+chain) || !strings.Contains(trimmed, "policy accept") {
				return fmt.Errorf("Hacocoon sandbox firewall chain %q has unsafe hook %q: %w", chain, trimmed, core.ErrIncompatibleState)
			}
			seenHooks[chain] = true
		case chain != "" && trimmed != "" && trimmed != "}" && !strings.HasPrefix(trimmed, "table "):
			seenRules[chain] = append(seenRules[chain], trimmed)
		}
	}
	for _, name := range []string{"input", "forward"} {
		if !seenHooks[name] {
			return fmt.Errorf("Hacocoon sandbox firewall is missing %s hook: %w", name, core.ErrIncompatibleState)
		}
		want := expectedRules[name]
		got := seenRules[name]
		if len(got) != len(want) {
			return fmt.Errorf("Hacocoon sandbox firewall %s rules = %q, want %q: %w", name, got, want, core.ErrIncompatibleState)
		}
		for i := range want {
			if got[i] != want[i] {
				return fmt.Errorf("Hacocoon sandbox firewall %s rule %d = %q, want %q: %w", name, i, got[i], want[i], core.ErrIncompatibleState)
			}
		}
	}
	return nil
}

func environmentBridgeName(ref string) string {
	return routedSandboxHostInterface(ref)
}

func environmentBridgeMAC(ref string) string {
	sum := sha256.Sum256([]byte("mac:" + ref))
	// Locally administered unicast MAC.
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3], sum[4])
}

func (r *Runtime) ensureEnvironmentBridge(ctx context.Context, ref string) (netip.Prefix, error) {
	bridge := environmentBridgeName(ref)
	show, err := r.runner.Run(ctx, "incus", "network", "show", bridge, "--project", sandboxBridgeResourceProject)
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "create", bridge,
			"ipv4.address=auto",
			"ipv4.nat=false",
			"ipv4.firewall=false",
			"ipv4.routing=true",
			"ipv6.address=none",
			"raw.dnsmasq=port=0",
			"--project", sandboxBridgeResourceProject,
		); createErr != nil {
			return netip.Prefix{}, fmt.Errorf("create dedicated Environment bridge %s: %w", bridge, createErr)
		}
	} else if strings.Contains(strings.ToLower(show.Stdout), "managed: false") {
		return netip.Prefix{}, fmt.Errorf("dedicated Environment bridge %s is unmanaged: %w", bridge, core.ErrIncompatibleState)
	}

	expected := map[string]string{
		"ipv4.nat":      "false",
		"ipv4.firewall": "false",
		"ipv4.routing":  "true",
		"ipv6.address":  "none",
		"raw.dnsmasq":   "port=0",
	}
	for key, want := range expected {
		got, getErr := r.runner.Run(ctx, "incus", "network", "get", bridge, key, "--project", sandboxBridgeResourceProject)
		if getErr != nil {
			return netip.Prefix{}, fmt.Errorf("verify Environment bridge %s %s: %w", bridge, key, getErr)
		}
		if strings.TrimSpace(got.Stdout) != want {
			return netip.Prefix{}, fmt.Errorf("Environment bridge %s %s=%q, want %q: %w", bridge, key, strings.TrimSpace(got.Stdout), want, core.ErrIncompatibleState)
		}
	}
	address, getErr := r.runner.Run(ctx, "incus", "network", "get", bridge, "ipv4.address", "--project", sandboxBridgeResourceProject)
	if getErr != nil {
		return netip.Prefix{}, fmt.Errorf("resolve Environment bridge %s IPv4 subnet: %w", bridge, getErr)
	}
	prefix, parseErr := netip.ParsePrefix(strings.TrimSpace(address.Stdout))
	if parseErr != nil || !prefix.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("Environment bridge %s has invalid IPv4 subnet %q: %w", bridge, strings.TrimSpace(address.Stdout), core.ErrIncompatibleState)
	}
	return prefix.Masked(), nil
}

// ensureRoutedSandboxSourceGuard now protects a dedicated Environment bridge.
// The bridge itself removes cross-Environment L2 reachability. An inet-family
// prerouting guard additionally rejects MAC spoofing and any source outside the
// Incus-assigned bridge subnet, avoiding CONFIG_NF_TABLES_BRIDGE entirely.
func (r *Runtime) ensureRoutedSandboxSourceGuard(ctx context.Context, ref, subnet string) error {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(subnet))
	if err != nil || !prefix.Addr().Is4() {
		return fmt.Errorf("invalid Environment bridge source guard subnet %q: %w", subnet, core.ErrInvalidArgument)
	}
	iface := environmentBridgeName(ref)
	table := routedSandboxGuardTable(ref)
	mac := environmentBridgeMAC(ref)

	shown, listErr := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, table)
	if listErr == nil {
		if _, err := r.runRoutedPrivileged(ctx, "nft", "delete", "table", sandboxRoutedFirewallFamily, table); err != nil {
			return fmt.Errorf("replace stale Environment source guard %s: %w", table, err)
		}
	} else if !nftTableMissing(shown) {
		return fmt.Errorf("inspect Environment source guard %s: %w", table, listErr)
	}

	created := false
	rollback := func() {
		if created {
			_, _ = r.runRoutedPrivileged(context.WithoutCancel(ctx), "nft", "delete", "table", sandboxRoutedFirewallFamily, table)
		}
	}
	if _, err := r.runRoutedPrivileged(ctx, "nft", "add", "table", sandboxRoutedFirewallFamily, table); err != nil {
		return fmt.Errorf("create Environment source guard %s: %w", table, err)
	}
	created = true
	commands := [][]string{
		{"add", "chain", sandboxRoutedFirewallFamily, table, "prerouting", "{", "type", "filter", "hook", "prerouting", "priority", "-300", ";", "policy", "accept", ";", "}"},
		{"add", "rule", sandboxRoutedFirewallFamily, table, "prerouting", "iifname", "\"" + iface + "\"", "ether", "saddr", "!=", mac, "drop"},
		{"add", "rule", sandboxRoutedFirewallFamily, table, "prerouting", "iifname", "\"" + iface + "\"", "ip", "saddr", "!=", prefix.String(), "drop"},
	}
	for _, args := range commands {
		if _, err := r.runRoutedPrivileged(ctx, "nft", args...); err != nil {
			rollback()
			return fmt.Errorf("configure Environment source guard %s: %w", table, err)
		}
	}
	return nil
}

func (r *Runtime) verifyRoutedSandboxSourceGuard(ctx context.Context, ref string, prefix netip.Prefix) error {
	table := routedSandboxGuardTable(ref)
	shown, err := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, table)
	if err != nil {
		return fmt.Errorf("inspect Environment source guard %s: %w", table, err)
	}
	return verifyRoutedSandboxSourceGuard(shown.Stdout, environmentBridgeName(ref), prefix.String())
}

func verifyRoutedSandboxSourceGuard(raw, iface, subnet string) error {
	chain := ""
	seenHook := false
	var rules []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "chain prerouting ") || trimmed == "chain prerouting {":
			chain = "prerouting"
		case strings.HasPrefix(trimmed, "chain "):
			return fmt.Errorf("Environment source guard contains unmanaged chain %q: %w", trimmed, core.ErrIncompatibleState)
		case strings.HasPrefix(trimmed, "type filter hook "):
			if chain != "prerouting" || !strings.Contains(trimmed, "hook prerouting") || !strings.Contains(trimmed, "policy accept") || (!strings.Contains(trimmed, "priority -300") && !strings.Contains(trimmed, "priority raw")) {
				return fmt.Errorf("Environment source guard has unsafe hook %q: %w", trimmed, core.ErrIncompatibleState)
			}
			seenHook = true
		case chain == "prerouting" && trimmed != "" && trimmed != "}" && !strings.HasPrefix(trimmed, "table "):
			rules = append(rules, trimmed)
		}
	}
	wantMAC := "iifname \"" + iface + "\" ether saddr != " + environmentBridgeMACFromInterface(iface) + " drop"
	wantIP := "iifname \"" + iface + "\" ip saddr != " + subnet + " drop"
	if !seenHook || len(rules) != 2 || rules[0] != wantMAC || rules[1] != wantIP {
		return fmt.Errorf("Environment source guard rules = %q hook=%t, want %q then %q: %w", rules, seenHook, wantMAC, wantIP, core.ErrIncompatibleState)
	}
	return nil
}

func environmentBridgeMACFromInterface(iface string) string {
	// Interface names are a deterministic hash of the Environment ref, but the
	// verifier does not have the ref. Derive a stable locally-administered MAC
	// from the bridge identity itself; addRoutedSandboxNIC uses the same helper.
	sum := sha256.Sum256([]byte("bridge-mac:" + iface))
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3], sum[4])
}

func (r *Runtime) removeRoutedSandboxSourceGuard(ctx context.Context, ref string) error {
	table := routedSandboxGuardTable(ref)
	shown, err := r.runRoutedPrivileged(ctx, "nft", "list", "table", sandboxRoutedFirewallFamily, table)
	if err == nil {
		if _, deleteErr := r.runRoutedPrivileged(ctx, "nft", "delete", "table", sandboxRoutedFirewallFamily, table); deleteErr != nil {
			return fmt.Errorf("delete Environment source guard %s: %w", table, deleteErr)
		}
	} else if !nftTableMissing(shown) {
		return fmt.Errorf("inspect Environment source guard %s before cleanup: %w", table, err)
	}

	bridge := environmentBridgeName(ref)
	if _, err := r.runner.Run(ctx, "incus", "network", "delete", bridge, "--project", sandboxBridgeResourceProject); err != nil {
		show, showErr := r.runner.Run(ctx, "incus", "network", "show", bridge, "--project", sandboxBridgeResourceProject)
		if showErr == nil || strings.TrimSpace(show.Stdout) != "" {
			return fmt.Errorf("delete dedicated Environment bridge %s: %w", bridge, err)
		}
	}
	return nil
}

func (r *Runtime) addRoutedSandboxNIC(ctx context.Context, ref string) error {
	if err := r.ensureRoutedSandboxHost(ctx); err != nil {
		return err
	}
	prefix, err := r.ensureEnvironmentBridge(ctx, ref)
	if err != nil {
		return err
	}
	iface := environmentBridgeName(ref)
	mac := environmentBridgeMACFromInterface(iface)
	args := []string{
		"config", "device", "add", ref, "eth0", "nic",
		"name=eth0",
		"network=" + iface,
		"hwaddr=" + mac,
		"security.port_isolation=true",
		"--project", r.project,
	}
	result, err := r.runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return fmt.Errorf("add dedicated-bridge sandbox NIC: %s: %w", reason, err)
		}
		return fmt.Errorf("add dedicated-bridge sandbox NIC: %w", err)
	}
	if err := r.ensureRoutedSandboxSourceGuard(ctx, ref, prefix.String()); err != nil {
		return fmt.Errorf("install Environment anti-spoofing guard for %s: %w", ref, err)
	}
	return nil
}

func (r *Runtime) verifyRoutedSandboxAntiSpoof(ctx context.Context, ref string) error {
	bridge := environmentBridgeName(ref)
	configured, err := r.runner.Run(ctx, "incus", "config", "device", "get", ref, "eth0", "network", "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify dedicated Environment bridge binding: %w", err)
	}
	if strings.TrimSpace(configured.Stdout) != bridge {
		return fmt.Errorf("Environment NIC network=%q, want %s: %w", strings.TrimSpace(configured.Stdout), bridge, core.ErrIncompatibleState)
	}
	mac, err := r.runner.Run(ctx, "incus", "config", "device", "get", ref, "eth0", "hwaddr", "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify Environment MAC identity: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(mac.Stdout)) != environmentBridgeMACFromInterface(bridge) {
		return fmt.Errorf("Environment NIC MAC=%q is not managed identity: %w", strings.TrimSpace(mac.Stdout), core.ErrIncompatibleState)
	}
	prefix, err := r.ensureEnvironmentBridge(ctx, ref)
	if err != nil {
		return err
	}
	if err := r.verifyRoutedSandboxSourceGuard(ctx, ref, prefix); err != nil {
		return fmt.Errorf("verify Environment source identity guard: %w", err)
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
	value := binary.BigEndian.Uint64(sum[:8]) & 0xffffffffffff
	return fmt.Sprintf("%s%012x", sandboxRoutedHostPrefix, value)
}

func routedSandboxGuardTable(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	value := binary.BigEndian.Uint64(sum[:8]) & 0xffffffffff
	return fmt.Sprintf("%s%010x", sandboxRoutedGuardPrefix, value)
}

// Compatibility helpers retained while routed-network-specific tests and old
// migrations are removed. They no longer drive Environment networking.
func (r *Runtime) allocateRoutedSandboxIPv4(ctx context.Context, ref string) (string, error) {
	result, err := r.runner.Run(ctx, "incus", "list", "--project", r.project, "--format", "json")
	if err != nil {
		return "", fmt.Errorf("inspect compatibility address allocations: %w", err)
	}
	var instances []struct {
		Name            string                       `json:"name"`
		Devices         map[string]map[string]string `json:"devices"`
		ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &instances); err != nil {
		return "", fmt.Errorf("decode compatibility address allocations: %w", err)
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
	return "", fmt.Errorf("compatibility address pool %s is exhausted: %w", sandboxRoutedGuestPool, core.ErrIncompatibleState)
}

func routedSandboxIPv4At(offset uint32) string {
	second := byte(18 + ((offset >> 16) & 0x1))
	third := byte((offset >> 8) & 0xff)
	fourth := byte(offset & 0xff)
	return netip.AddrFrom4([4]byte{198, second, third, fourth}).String()
}
