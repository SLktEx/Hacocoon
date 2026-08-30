package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

const (
	sandboxProfile         = "haco-sandbox"
	sandboxNetwork         = "haco-sandbox0"
	sandboxEgressACL       = "haco-sandbox-egress"
	sandboxResourceProject = "default"
	sandboxDNSMasq         = "port=0"
	sandboxProxyRule       = "hacocoon-standard-egress-proxy-v1"
)

var sandboxNIC = map[string]string{
	"type":                                  "nic",
	"name":                                  "eth0",
	"network":                               sandboxNetwork,
	"security.acls":                         sandboxEgressACL,
	"security.acls.default.ingress.action":  "reject",
	"security.acls.default.egress.action":   "reject",
	"security.acls.default.ingress.logged":  "true",
	"security.acls.default.egress.logged":   "true",
	"security.ipv4_filtering":               "true",
	"security.ipv6_filtering":               "true",
	"security.mac_filtering":                "true",
	"security.port_isolation":               "true",
}

// ensureSandboxNetwork prepares the shared Incus substrate so the Standard
// hostname-aware proxy is the only ordinary outbound transport. The bridge DNS
// listener is disabled to avoid a DNS exfiltration side channel; DHCP remains
// available because dnsmasq port=0 disables DNS service only. The NIC ACL
// continues to reject unmatched egress and permits only the bridge gateway's
// Standard proxy listener.
func (r *Runtime) ensureSandboxNetwork(ctx context.Context) error {
	if r == nil || r.runner == nil {
		return core.ErrInvalidArgument
	}
	gateway, err := r.ensureSandboxBridge(ctx)
	if err != nil {
		return err
	}
	if err := r.ensureSandboxACL(ctx, gateway); err != nil {
		return err
	}
	return r.ensureSandboxProfile(ctx, gateway)
}

func (r *Runtime) ensureSandboxBridge(ctx context.Context) (netip.Addr, error) {
	if _, err := r.runner.Run(ctx, "incus", "network", "show", sandboxNetwork, "--project", sandboxResourceProject); err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "create", sandboxNetwork,
			"ipv4.address=auto",
			"ipv4.nat=true",
			"ipv4.firewall=true",
			"ipv4.routing=true",
			"ipv6.address=none",
			"raw.dnsmasq="+sandboxDNSMasq,
			"--project", sandboxResourceProject,
		); createErr != nil {
			return netip.Addr{}, fmt.Errorf("create Hacocoon sandbox network: %w", createErr)
		}
	}

	rawDNS, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "raw.dnsmasq", "--project", sandboxResourceProject)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("verify Hacocoon sandbox network raw.dnsmasq: %w", err)
	}
	switch strings.TrimSpace(rawDNS.Stdout) {
	case "":
		if _, err := r.runner.Run(ctx, "incus", "network", "set", sandboxNetwork, "raw.dnsmasq="+sandboxDNSMasq, "--project", sandboxResourceProject); err != nil {
			return netip.Addr{}, fmt.Errorf("disable Hacocoon sandbox bridge DNS: %w", err)
		}
	case sandboxDNSMasq:
	default:
		return netip.Addr{}, fmt.Errorf("Hacocoon sandbox network raw.dnsmasq contains unmanaged configuration: %w", core.ErrIncompatibleState)
	}

	checks := map[string]func(string) bool{
		"ipv4.address": func(v string) bool {
			v = strings.TrimSpace(v)
			return v != "" && v != "none"
		},
		"ipv4.nat":      func(v string) bool { return strings.TrimSpace(v) == "true" },
		"ipv4.firewall": func(v string) bool { return strings.TrimSpace(v) == "true" },
		"ipv4.routing":  func(v string) bool { return strings.TrimSpace(v) == "true" },
		"ipv6.address":  func(v string) bool { return strings.TrimSpace(v) == "none" },
		"raw.dnsmasq":   func(v string) bool { return strings.TrimSpace(v) == sandboxDNSMasq },
	}
	values := map[string]string{}
	keys := make([]string, 0, len(checks))
	for key := range checks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		got, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, key, "--project", sandboxResourceProject)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("verify Hacocoon sandbox network %s: %w", key, err)
		}
		if !checks[key](got.Stdout) {
			return netip.Addr{}, fmt.Errorf("Hacocoon sandbox network %s has unsafe value %q: %w", key, strings.TrimSpace(got.Stdout), core.ErrIncompatibleState)
		}
		values[key] = strings.TrimSpace(got.Stdout)
	}
	prefix, err := netip.ParsePrefix(values["ipv4.address"])
	if err != nil || !prefix.Addr().Is4() || prefix.Addr().IsUnspecified() || prefix.Addr().IsLoopback() {
		return netip.Addr{}, fmt.Errorf("Hacocoon sandbox network has invalid IPv4 gateway %q: %w", values["ipv4.address"], core.ErrIncompatibleState)
	}
	return prefix.Addr(), nil
}

func (r *Runtime) ensureSandboxACL(ctx context.Context, gateway netip.Addr) error {
	shown, err := r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
	created := false
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "acl", "create", sandboxEgressACL, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox ACL: %w", createErr)
		}
		created = true
	}
	if created || emptyACL(shown.Stdout) {
		destination := netip.PrefixFrom(gateway, 32).String()
		if _, addErr := r.runner.Run(ctx, "incus", "network", "acl", "rule", "add", sandboxEgressACL, "egress",
			"action=allow",
			"state=enabled",
			"description="+sandboxProxyRule,
			"destination="+destination,
			"protocol=tcp",
			"destination_port="+strconv.Itoa(egressapp.DefaultProxyPort),
			"--project", sandboxResourceProject,
		); addErr != nil {
			return fmt.Errorf("add Hacocoon Standard egress proxy ACL rule: %w", addErr)
		}
		shown, err = r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox ACL: %w", err)
		}
	}
	if !managedProxyACL(shown.Stdout, gateway) {
		return fmt.Errorf("Hacocoon sandbox ACL drifted from proxy-only egress policy: %w", core.ErrIncompatibleState)
	}
	return nil
}

func emptyACL(raw string) bool {
	ingressEmpty := false
	egressEmpty := false
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "ingress: []":
			ingressEmpty = true
		case "egress: []":
			egressEmpty = true
		}
	}
	return ingressEmpty && egressEmpty
}

func managedProxyACL(raw string, gateway netip.Addr) bool {
	ingressEmpty := false
	inEgress := false
	rules := 0
	fields := map[string]string{}
	valid := true
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "ingress: []" {
			ingressEmpty = true
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			if trimmed == "egress:" {
				inEgress = true
				continue
			}
			if inEgress && !strings.HasPrefix(trimmed, "- ") {
				inEgress = false
			}
		}
		if !inEgress || trimmed == "" {
			continue
		}
		entry := trimmed
		if strings.HasPrefix(entry, "- ") {
			rules++
			entry = strings.TrimSpace(strings.TrimPrefix(entry, "- "))
		}
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			valid = false
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if _, exists := fields[key]; exists {
			valid = false
			continue
		}
		fields[key] = value
	}
	if !valid || !ingressEmpty || rules != 1 || len(fields) != 6 {
		return false
	}
	destination := netip.PrefixFrom(gateway, 32).String()
	return fields["action"] == "allow" &&
		fields["state"] == "enabled" &&
		fields["description"] == sandboxProxyRule &&
		(fields["destination"] == destination || fields["destination"] == gateway.String()) &&
		fields["protocol"] == "tcp" &&
		fields["destination_port"] == strconv.Itoa(egressapp.DefaultProxyPort)
}

func sandboxProxyEnvironment(gateway netip.Addr) map[string]string {
	proxy := "http://" + gateway.String() + ":" + strconv.Itoa(egressapp.DefaultProxyPort)
	noProxy := "127.0.0.1,localhost,::1"
	return map[string]string{
		"environment.HTTP_PROXY":  proxy,
		"environment.HTTPS_PROXY": proxy,
		"environment.NO_PROXY":    noProxy,
		"environment.http_proxy":  proxy,
		"environment.https_proxy": proxy,
		"environment.no_proxy":    noProxy,
	}
}

func (r *Runtime) ensureSandboxProfile(ctx context.Context, gateway netip.Addr) error {
	shown, err := r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
	created := false
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "profile", "create", sandboxProfile, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox profile: %w", createErr)
		}
		created = true
		args := []string{"profile", "device", "add", sandboxProfile, "eth0", "nic"}
		for _, key := range []string{
			"name", "network", "security.acls", "security.acls.default.ingress.action", "security.acls.default.egress.action",
			"security.acls.default.ingress.logged", "security.acls.default.egress.logged", "security.ipv4_filtering",
			"security.ipv6_filtering", "security.mac_filtering", "security.port_isolation",
		} {
			args = append(args, key+"="+sandboxNIC[key])
		}
		args = append(args, "--project", sandboxResourceProject)
		if _, addErr := r.runner.Run(ctx, "incus", args...); addErr != nil {
			return fmt.Errorf("configure Hacocoon sandbox profile NIC: %w", addErr)
		}
	}

	expectedConfig := sandboxProxyEnvironment(gateway)
	if created {
		if err := r.setSandboxProfileConfig(ctx, expectedConfig); err != nil {
			return err
		}
		shown, err = r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox profile: %w", err)
		}
	}

	var profile struct {
		Config  map[string]string            `json:"config"`
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(shown.Stdout), &profile); err != nil {
		return fmt.Errorf("decode Hacocoon sandbox profile: %w", err)
	}
	if !created && len(profile.Config) == 0 {
		if err := r.setSandboxProfileConfig(ctx, expectedConfig); err != nil {
			return err
		}
		shown, err = r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
		if err != nil {
			return fmt.Errorf("verify migrated Hacocoon sandbox profile: %w", err)
		}
		if err := json.Unmarshal([]byte(shown.Stdout), &profile); err != nil {
			return fmt.Errorf("decode migrated Hacocoon sandbox profile: %w", err)
		}
	}
	if !sameStringMap(profile.Config, expectedConfig) || len(profile.Devices) != 1 {
		return fmt.Errorf("Hacocoon sandbox profile contains unmanaged config/devices: %w", core.ErrIncompatibleState)
	}
	got, ok := profile.Devices["eth0"]
	if !ok || !sameStringMap(got, sandboxNIC) {
		return fmt.Errorf("Hacocoon sandbox profile NIC drifted from managed policy: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) setSandboxProfileConfig(ctx context.Context, config map[string]string) error {
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := []string{"profile", "set", sandboxProfile}
	for _, key := range keys {
		args = append(args, key+"="+config[key])
	}
	args = append(args, "--project", sandboxResourceProject)
	if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
		return fmt.Errorf("configure Hacocoon sandbox proxy environment: %w", err)
	}
	return nil
}

func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range b {
		if a[key] != value {
			return false
		}
	}
	return true
}
