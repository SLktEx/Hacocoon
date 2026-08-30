package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	sandboxProfile         = "haco-sandbox"
	sandboxNetwork         = "haco-sandbox0"
	sandboxEgressACL       = "haco-sandbox-egress"
	sandboxResourceProject = "default"
)

var sandboxNIC = map[string]string{
	"type":                                  "nic",
	"name":                                  "eth0",
	"network":                               sandboxNetwork,
	"security.acls":                         sandboxEgressACL,
	"security.acls.default.ingress.action": "reject",
	"security.acls.default.egress.action":  "reject",
	"security.acls.default.ingress.logged": "true",
	"security.acls.default.egress.logged":  "true",
	"security.ipv4_filtering":               "true",
	"security.ipv6_filtering":               "true",
	"security.mac_filtering":                "true",
	"security.port_isolation":               "true",
}

// ensureSandboxNetwork prepares the shared Incus network substrate used by
// Hacocoon environments. The bridge provides DHCP but its DNS listener is
// disabled; the empty ACL attached directly to each sandbox NIC makes unmatched
// ingress and egress fail closed. Domain-aware egress authorization is handled
// by the host broker rather than approximated with DNS-to-IP ACLs.
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
	return r.ensureSandboxDNSDisabled(ctx)
}

func (r *Runtime) ensureSandboxACL(ctx context.Context) error {
	shown, err := r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "network", "acl", "create", sandboxEgressACL, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox ACL: %w", createErr)
		}
		shown, err = r.runner.Run(ctx, "incus", "network", "acl", "show", sandboxEgressACL, "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox ACL: %w", err)
		}
	}
	if !emptyACL(shown.Stdout) {
		return fmt.Errorf("Hacocoon sandbox ACL contains unmanaged allow/reject rules: %w", core.ErrIncompatibleState)
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

func (r *Runtime) ensureSandboxProfile(ctx context.Context) error {
	shown, err := r.runner.Run(ctx, "incus", "profile", "show", sandboxProfile, "--project", sandboxResourceProject, "--format", "json")
	if err != nil {
		if _, createErr := r.runner.Run(ctx, "incus", "profile", "create", sandboxProfile, "--project", sandboxResourceProject); createErr != nil {
			return fmt.Errorf("create Hacocoon sandbox profile: %w", createErr)
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
		if _, addErr := r.runner.Run(ctx, "incus", args...); addErr != nil {
			return fmt.Errorf("configure Hacocoon sandbox profile NIC: %w", addErr)
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
	if len(profile.Config) != 0 || len(profile.Devices) != 1 {
		return fmt.Errorf("Hacocoon sandbox profile contains unmanaged config/devices: %w", core.ErrIncompatibleState)
	}
	got, ok := profile.Devices["eth0"]
	if !ok || !sameStringMap(got, sandboxNIC) {
		return fmt.Errorf("Hacocoon sandbox profile NIC drifted from managed policy: %w", core.ErrIncompatibleState)
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
