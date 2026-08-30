package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const sandboxDNSDisabledConfig = "port=0"

// ensureSandboxDNSDisabled keeps DHCP on the managed bridge but disables
// dnsmasq's DNS listener. Proxy-aware workloads send hostnames to the trusted
// egress broker; guest DNS must not become an unaudited exfiltration channel.
func (r *Runtime) ensureSandboxDNSDisabled(ctx context.Context) error {
	got, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "raw.dnsmasq", "--project", sandboxResourceProject)
	if err != nil {
		return fmt.Errorf("inspect Hacocoon sandbox DNS configuration: %w", err)
	}
	current := strings.TrimSpace(got.Stdout)
	switch current {
	case sandboxDNSDisabledConfig:
		return nil
	case "":
		if _, err := r.runner.Run(ctx, "incus", "network", "set", sandboxNetwork, "raw.dnsmasq="+sandboxDNSDisabledConfig, "--project", sandboxResourceProject); err != nil {
			return fmt.Errorf("disable Hacocoon sandbox DNS: %w", err)
		}
		verified, err := r.runner.Run(ctx, "incus", "network", "get", sandboxNetwork, "raw.dnsmasq", "--project", sandboxResourceProject)
		if err != nil {
			return fmt.Errorf("verify Hacocoon sandbox DNS configuration: %w", err)
		}
		if strings.TrimSpace(verified.Stdout) != sandboxDNSDisabledConfig {
			return fmt.Errorf("Hacocoon sandbox DNS disablement did not persist: %w", core.ErrIncompatibleState)
		}
		return nil
	default:
		return fmt.Errorf("Hacocoon sandbox has unmanaged raw.dnsmasq configuration %q; refusing to overwrite it: %w", current, core.ErrIncompatibleState)
	}
}
