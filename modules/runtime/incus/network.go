package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	sandboxProfile         = "haco-sandbox"
	sandboxNetwork         = "haco-sandbox0"
	sandboxEgressACL       = "haco-sandbox-egress"
	sandboxResourceProject = "default"
	sandboxProxyRuleDesc   = "Hacocoon Standard egress proxy"
)

var sandboxNIC = map[string]string{
	"type":                                 "nic",
	"name":                                 "eth0",
	"network":                              sandboxNetwork,
	"security.acls":                        sandboxEgressACL,
	"security.acls.default.ingress.action": "reject",
	"security.acls.default.egress.action":  "reject",
	"security.acls.default.ingress.logged": "true",
	"security.acls.default.egress.logged":  "true",
	"security.ipv4_filtering":              "true",
	"security.ipv6_filtering":              "true",
	"security.mac_filtering":               "true",
	"security.port_isolation":              "true",
}

// ensureSandboxNetwork prepares the shared Incus network substrate used by
// Hacocoon environments. DHCP remains available on the managed bridge, but the
// bridge DNS listener is disabled and the NIC ACL allows only the Standard
// egress proxy. Hostname authorization remains in the proxy/Capability layer;
// the IP-layer ACL is only a transport guard against broker bypass.
func (r *Runtime) ensureSandboxNetwork(ctx context.Context) error {
	if r == nil || r.runner == nil {
		return core.ErrInvalidArgument
	}
	if err := r.ensureSandboxBridge(ctx); err != nil {
		return err
	}
	if err := r.ensureSandboxACL(ctx); err != nil {
		return err
	}
	return r.ensureSandboxProfile(ctx)
}

func (r *Runtime) ensureSandboxBridge(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "incus", "network", "show", sandboxNetwork, "--project", sandboxResourceProject); err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "create", sandboxNetwork,
			"ipv4.address=auto",
			"ipv4.nat=true",
			"ipv4.firewall=true",
			"ipv4.routing=true",
			"ipv6.address=none",
			"raw.dnsmasq=port=0",
			"--project", sandboxResourceProject,
		); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox network: %w", createErr)
		}
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
	}
	for key, valid := range checks {
		got, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, key, "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox network %s: %w", key, err)
		}
		if !valid(got.Stdout) {
			return fmt.Errorf("Hacocoon sandbox network %s has unsafe value %q: %w", key, strings.TrimSpace(got.Stdout), core.ErrIncompatibleState)
		}
	}

	dns, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "raw.dnsmasq", "--project", sandboxResourceProject)
	if err != nil {
		return fmt.Errorf("verify Hacocoon sandbox DNS disablement: %w", err)
	}
	switch strings.TrimSpace(dns.Stdout) {
	case "port=0":
		return nil
	case "":
		if _, err := r.runner.Run(ctx, "incus", "network", "set", sandboxNetwork, "raw.dnsmasq=port=0", "--project", sandboxResourceProject); err != nil {
			return fmt.Errorf("disable Hacocoon bridge DNS: %w", err)
		}
		got, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "raw.dnsmasq", "--project", sandboxResourceProject)
		if err != nil || strings.TrimSpace(got.Stdout) != "port=0" {
			if err != nil {
				return fmt.Errorf("verify disabled Hacocoon bridge DNS: %w", err)
			}
			return fmt.Errorf("Hacocoon bridge DNS disablement did not persist: %w", core.ErrIncompatibleState)
		}
		return nil
	default:
		return fmt.Errorf("Hacocoon sandbox network has unmanaged raw.dnsmasq value %q: %w", strings.TrimSpace(dns.Stdout), core.ErrIncompatibleState)
	}
}

func (r *Runtime) ensureSandboxACL(ctx context.Context) error {
	shown, err := r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "acl", "create", sandboxEgressACL, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox ACL: %w", createErr)
		}
		shown, err = r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify newly-created Hacocoon sandbox ACL: %w", err)
		}
	}

	gateway, err := r.sandboxGateway(ctx)
	if err != nil {
		return err
	}
	destination := gateway.String() + "/32"
	if emptyACL(shown.Stdout) {
		if _, err := r.runner.Run(ctx, "incus", "network", "acl", "rule", "add", sandboxEgressACL, "egress",
			"action=allow",
			"state=enabled",
			"destination="+destination,
			"protocol=tcp",
			"destination_port="+strconv.Itoa(sandboxEgressProxyPort),
			"description="+sandboxProxyRuleDesc,
			"--project", sandboxResourceProject,
		); err != nil {
			return fmt.Errorf("allow Hacocoon Standard egress proxy transport: %w", err)
		}
		shown, err = r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox ACL after proxy rule: %w", err)
		}
	}
	if !managedProxyACL(shown.Stdout, destination) {
		return fmt.Errorf("Hacocoon sandbox ACL contains unmanaged or unsafe rules: %w", core.ErrIncompatibleState)
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

func managedProxyACL(raw, destination string) bool {
	ingressEmpty := false
	inEgress := false
	ruleCount := 0
	rule := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "egress:":
			inEgress = true
			continue
		case strings.HasPrefix(trimmed, "ingress:"):
			inEgress = false
			ingressEmpty = trimmed == "ingress: []"
			continue
		case !inEgress || trimmed == "":
			continue
		}
		entry := trimmed
		if strings.HasPrefix(entry, "- ") {
			ruleCount++
			entry = strings.TrimSpace(strings.TrimPrefix(entry, "- "))
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || ruleCount != 1 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		rule[key] = value
	}
	if !ingressEmpty || ruleCount != 1 {
		return false
	}
	expected := map[string]string{
		"action":           "allow",
		"state":            "enabled",
		"destination":      destination,
		"protocol":         "tcp",
		"destination_port": strconv.Itoa(sandboxEgressProxyPort),
		"description":      sandboxProxyRuleDesc,
	}
	for key, value := range expected {
		if rule[key] != value {
			return false
		}
	}
	for key, value := range rule {
		if _, ok := expected[key]; ok {
			continue
		}
		if value != "" {
			return false
		}
	}
	return true
}

func (r *Runtime) ensureSandboxProfile(ctx context.Context) error {
	expectedConfig, err := r.sandboxProfileConfig(ctx)
	if err != nil {
		return err
	}
	shown, err := r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "profile", "create", sandboxProfile, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox profile: %w", createErr)
		}
		if err := r.setSandboxProfileConfig(ctx, expectedConfig); err != nil {
			return err
		}
		args := []string{"profile", "device", "add", sandboxProfile, "eth0", "nic"}
		for _, key := range []string{
			"name", "network", "security.acls", "security.acls.default.ingress.action", "security.acls.default.egress.action",
			"security.acls.default.ingress.logged", "security.acls.default.egress.logged", "security.ipv4_filtering",
			"security.ipv6_filtering", "security.mac_filtering", "security.port_isolation",
		} {
			args = append(args, key+"="+sandboxNIC[key])
		}
		args = append(args, "--project", sandboxResourceProject)
		addResult, addErr := r.runner.Run(ctx, "incus", args...)
		if addErr != nil || addResult.ExitCode != 0 {
			return fmt.Errorf("configure Hacocoon sandbox profile NIC: %w", commandResultError(addResult, addErr))
		}
		shown, err = r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox profile: %w", err)
		}
	}

	profile, err := decodeSandboxProfile(shown.Stdout)
	if err != nil {
		return err
	}
	if len(profile.Config) == 0 {
		if err := r.setSandboxProfileConfig(ctx, expectedConfig); err != nil {
			return err
		}
		shown, err = r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
		if err != nil {
			return fmt.Errorf("verify migrated Hacocoon sandbox profile: %w", err)
		}
		profile, err = decodeSandboxProfile(shown.Stdout)
		if err != nil {
			return err
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

type sandboxProfileState struct {
	Config  map[string]string            `json:"config"`
	Devices map[string]map[string]string `json:"devices"`
}

func decodeSandboxProfile(raw string) (sandboxProfileState, error) {
	var profile sandboxProfileState
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return sandboxProfileState{}, fmt.Errorf("decode Hacocoon sandbox profile: %w", err)
	}
	if profile.Config == nil {
		profile.Config = map[string]string{}
	}
	if profile.Devices == nil {
		profile.Devices = map[string]map[string]string{}
	}
	return profile, nil
}

func (r *Runtime) sandboxProfileConfig(ctx context.Context) (map[string]string, error) {
	proxyURL, err := r.sandboxProxyURL(ctx)
	if err != nil {
		return nil, err
	}
	const noProxy = "localhost,127.0.0.1,::1"
	return map[string]string{
		"environment.HTTP_PROXY":  proxyURL,
		"environment.HTTPS_PROXY": proxyURL,
		"environment.NO_PROXY":    noProxy,
		"environment.http_proxy":  proxyURL,
		"environment.https_proxy": proxyURL,
		"environment.no_proxy":    noProxy,
	}, nil
}

func (r *Runtime) setSandboxProfileConfig(ctx context.Context, config map[string]string) error {
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := r.runner.Run(ctx, "incus", "profile", "set", sandboxProfile, key+"="+config[key], "--project", sandboxResourceProject); err != nil {
			return fmt.Errorf("configure Hacocoon sandbox profile %s: %w", key, err)
		}
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
