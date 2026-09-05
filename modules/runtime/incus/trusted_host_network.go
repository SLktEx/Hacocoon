package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"reflect"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostNetwork = "haco-host0"
const trustedHostNetworkOwner = "trusted-host-network-v1"

func (r *Runtime) ensureTrustedHostNetworkAndRunning(ctx context.Context, state, rootPool string) error {
	if err := r.ensureTrustedHostNetwork(ctx); err != nil {
		return err
	}
	state, err := r.reconcileTrustedHostNIC(ctx, state, rootPool)
	if err != nil {
		return err
	}
	return r.ensureTrustedHostRunning(ctx, state)
}

var trustedHostNetworkConfig = map[string]string{
	environmentNetworkOwnerKey: trustedHostNetworkOwner,
	"ipv4.nat":                 "true", "ipv4.firewall": "true", "ipv4.routing": "true", "ipv4.dhcp": "true",
	"ipv6.address": "none", "dns.mode": "managed",
}

type trustedNetwork struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Managed bool              `json:"managed"`
	Config  map[string]string `json:"config"`
	UsedBy  []string          `json:"used_by"`
}

// Only the controller's Incus adapter owns trusted-host networking. No default
// profile, installer storage probe, or Environment network selects this bridge.
func (r *Runtime) ensureTrustedHostNetwork(ctx context.Context) error {
	inspect := func() (*trustedNetwork, error) {
		result, err := r.runner.Run(ctx, "incus", "network", "list", "--project", sandboxResourceProject, "--format", "json")
		if err != nil {
			return nil, fmt.Errorf("inspect trusted-host networks: %w", err)
		}
		var networks []trustedNetwork
		if result.StdoutTruncated || json.Unmarshal([]byte(result.Stdout), &networks) != nil || networks == nil {
			return nil, fmt.Errorf("invalid trusted-host network inventory: %w", core.ErrIncompatibleState)
		}
		var found *trustedNetwork
		for i := range networks {
			if networks[i].Name == trustedHostNetwork {
				if found != nil {
					return nil, fmt.Errorf("duplicate trusted-host network: %w", core.ErrIncompatibleState)
				}
				found = &networks[i]
			}
		}
		return found, nil
	}
	network, err := inspect()
	if err != nil {
		return err
	}
	if network == nil {
		_, createErr := r.runner.Run(ctx, "incus", "network", "create", trustedHostNetwork,
			"--type", "bridge", "ipv4.address=auto", "ipv4.nat=true", "ipv4.firewall=true",
			"ipv4.routing=true", "ipv4.dhcp=true", "ipv6.address=none", "dns.mode=managed",
			environmentNetworkOwnerKey+"="+trustedHostNetworkOwner, "--project", sandboxResourceProject)
		// Ownership is recorded by the create itself. Never delete a persistent
		// bridge after a failed readback; retry must verify the exact owned object.
		network, err = inspect()
		if err != nil || network == nil {
			return errors.Join(fmt.Errorf("create/verify trusted-host network: %w", core.ErrIncompatibleState), createErr, err)
		}
	}
	prefix, err := verifyTrustedHostNetwork(*network, r.project)
	if err != nil {
		return err
	}
	return r.ensureTrustedHostForwarding(ctx, prefix)
}

func verifyTrustedHostNetwork(network trustedNetwork, project string) (netip.Prefix, error) {
	invalid := func() (netip.Prefix, error) {
		return netip.Prefix{}, fmt.Errorf("trusted-host network ownership/configuration mismatch: %w", core.ErrIncompatibleState)
	}
	if network.Name != trustedHostNetwork || network.Type != "bridge" || !network.Managed {
		return invalid()
	}
	for key, want := range trustedHostNetworkConfig {
		if network.Config[key] != want {
			return invalid()
		}
	}
	for key, value := range network.Config {
		if _, known := trustedHostNetworkConfig[key]; known {
			continue
		}
		if key == "ipv4.address" || key == "volatile.bridge.hwaddr" || strings.HasPrefix(key, "user.") {
			continue
		}
		// Unknown active routing, DNS, external interfaces and ACL configuration
		// are not silently adopted as trusted infrastructure.
		if value != "" {
			return invalid()
		}
	}
	prefix, err := netip.ParsePrefix(network.Config["ipv4.address"])
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() < 16 || prefix.Bits() > 28 {
		return invalid()
	}
	for _, consumer := range network.UsedBy {
		u, err := url.Parse(consumer)
		if err != nil || u.IsAbs() || u.Host != "" || u.Path != "/1.0/instances/"+trustedHostName || u.Query().Get("project") != project {
			return invalid()
		}
	}
	return prefix.Masked(), nil
}

// Docker's DOCKER-USER chain is its documented extension point. Incus accepts
// cannot override another forward hook's DROP. Grant only this verified bridge
// outbound forwarding and established replies; never change global policy or
// add rules for sandbox bridges. Reconcile before each trusted-host entry.
func (r *Runtime) ensureTrustedHostForwarding(ctx context.Context, subnet netip.Prefix) error {
	result, err := r.runRoutedPrivileged(ctx, "iptables", "-w", "5", "-S")
	if err != nil {
		return fmt.Errorf("inspect trusted-host forwarding: %w", err)
	}
	if result.StdoutTruncated {
		return fmt.Errorf("truncated forwarding inventory: %w", core.ErrIncompatibleState)
	}
	policy, dockerUser, dockerJump := "", false, false
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "-P" && fields[1] == "FORWARD" {
			policy = fields[2]
		}
		if line == "-N DOCKER-USER" {
			dockerUser = true
		}
		if line == "-A FORWARD -j DOCKER-USER" {
			dockerJump = true
		}
	}
	if policy != "ACCEPT" && policy != "DROP" {
		return fmt.Errorf("unknown forwarding policy: %w", core.ErrIncompatibleState)
	}
	if !dockerUser || !dockerJump {
		if policy == "ACCEPT" {
			return nil
		}
		return fmt.Errorf("FORWARD DROP without Docker's forwarding extension point: %w", core.ErrIncompatibleState)
	}
	rules := [][]string{
		{"-i", trustedHostNetwork, "-s", subnet.String(), "-m", "comment", "--comment", "hacocoon-trusted-host-outbound-v1", "-j", "ACCEPT"},
		{"-o", trustedHostNetwork, "-d", subnet.String(), "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-m", "comment", "--comment", "hacocoon-trusted-host-reply-v1", "-j", "ACCEPT"},
	}
	for _, rule := range rules {
		check := append([]string{"-w", "5", "-C", "DOCKER-USER"}, rule...)
		if _, err := r.runRoutedPrivileged(ctx, "iptables", check...); err == nil {
			continue
		}
		insert := append([]string{"-w", "5", "-I", "DOCKER-USER", "1"}, rule...)
		_, insertErr := r.runRoutedPrivileged(ctx, "iptables", insert...)
		if _, err := r.runRoutedPrivileged(ctx, "iptables", check...); err != nil {
			return errors.Join(fmt.Errorf("verify scoped trusted-host forwarding: %w", err), insertErr)
		}
	}
	return nil
}

type trustedHostNetworkState struct {
	Name            string                       `json:"name"`
	Config          map[string]string            `json:"config"`
	Devices         map[string]map[string]string `json:"devices"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	Profiles        []string                     `json:"profiles"`
}

func (r *Runtime) readTrustedHostNetworkState(ctx context.Context) (trustedHostNetworkState, error) {
	var state trustedHostNetworkState
	result, err := r.runner.Run(ctx, "incus", "query", "/1.0/instances/"+trustedHostName+"?project="+url.QueryEscape(r.project))
	if err != nil {
		return state, fmt.Errorf("inspect trusted-host NIC: %w", err)
	}
	if result.StdoutTruncated || json.Unmarshal([]byte(result.Stdout), &state) != nil || state.Name != trustedHostName || state.Config[trustedHostRoleKey] != trustedHostRoleValue || state.Devices == nil || state.ExpandedDevices == nil {
		return state, fmt.Errorf("invalid trusted-host NIC inventory: %w", core.ErrIncompatibleState)
	}
	return state, nil
}

func trustedHostNIC(network string) map[string]string {
	return map[string]string{"type": "nic", "name": "eth0", "network": network}
}

func verifyOnlyTrustedHostNIC(devices map[string]map[string]string, network string) bool {
	if !reflect.DeepEqual(devices["eth0"], trustedHostNIC(network)) {
		return false
	}
	for name, device := range devices {
		if name != "eth0" && device["type"] == "nic" {
			return false
		}
	}
	return true
}

// The one supported migration is our owned host's former default-profile NIC.
// Keep its root disk and UUID, stop gracefully before replugging, and never
// modify the shared profile/bridge or delete old storage. Partial migration is
// resumable from the explicit local NIC with the old profile still attached.
func (r *Runtime) reconcileTrustedHostNIC(ctx context.Context, currentState, rootPool string) (string, error) {
	state, err := r.readTrustedHostNetworkState(ctx)
	if err != nil {
		return currentState, err
	}
	if len(state.Profiles) == 0 && verifyOnlyTrustedHostNIC(state.Devices, trustedHostNetwork) && verifyOnlyTrustedHostNIC(state.ExpandedDevices, trustedHostNetwork) {
		return currentState, nil
	}
	root := map[string]string{"type": "disk", "path": "/", "pool": rootPool}
	legacyNIC := len(state.Devices["eth0"]) == 0 && verifyOnlyTrustedHostNIC(state.ExpandedDevices, "incusbr0")
	resumableNIC := verifyOnlyTrustedHostNIC(state.Devices, trustedHostNetwork) && verifyOnlyTrustedHostNIC(state.ExpandedDevices, trustedHostNetwork)
	if !reflect.DeepEqual(state.Profiles, []string{"default"}) || !reflect.DeepEqual(state.Devices["root"], root) || (!legacyNIC && !resumableNIC) {
		return currentState, fmt.Errorf("unmanaged trusted-host profiles/NIC: %w", core.ErrIncompatibleState)
	}
	for name, device := range state.ExpandedDevices {
		if name != "eth0" && !reflect.DeepEqual(device, state.Devices[name]) {
			return currentState, fmt.Errorf("unmanaged inherited trusted-host device: %w", core.ErrIncompatibleState)
		}
	}
	if currentState == "RUNNING" {
		if _, err := r.runner.Run(ctx, "incus", "stop", trustedHostName, "--timeout", "30", "--project", r.project); err != nil {
			return currentState, fmt.Errorf("stop trusted host for NIC migration: %w", err)
		}
		currentState = "STOPPED"
	} else if currentState != "STOPPED" {
		return currentState, fmt.Errorf("unsupported trusted-host state for NIC migration: %w", core.ErrIncompatibleState)
	}
	if legacyNIC {
		if _, err := r.runner.Run(ctx, "incus", "config", "device", "override", trustedHostName, "eth0", "network="+trustedHostNetwork, "--project", r.project); err != nil {
			return currentState, fmt.Errorf("migrate trusted-host NIC: %w", err)
		}
		verified, err := r.readTrustedHostNetworkState(ctx)
		if err != nil || !verifyOnlyTrustedHostNIC(verified.Devices, trustedHostNetwork) {
			return currentState, errors.Join(fmt.Errorf("trusted-host NIC migration did not persist: %w", core.ErrIncompatibleState), err)
		}
	}
	if _, err := r.runner.Run(ctx, "incus", "profile", "remove", trustedHostName, "default", "--project", r.project); err != nil {
		return currentState, fmt.Errorf("detach legacy trusted-host profile: %w", err)
	}
	verified, err := r.readTrustedHostNetworkState(ctx)
	if err != nil || len(verified.Profiles) != 0 || !verifyOnlyTrustedHostNIC(verified.ExpandedDevices, trustedHostNetwork) || !reflect.DeepEqual(verified.Devices["root"], root) {
		return currentState, errors.Join(fmt.Errorf("trusted-host network migration did not converge: %w", core.ErrIncompatibleState), err)
	}
	return currentState, nil
}
