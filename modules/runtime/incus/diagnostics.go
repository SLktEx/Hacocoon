package incus

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

// A running Incus instance can precede its DNS service and DHCP lease. Wait
// only for these local, read-only prerequisites; never retry an external probe.
const trustedNetworkStartupProbe = "until systemctl is-active --quiet systemd-resolved.service && ip -4 route show default | grep -q '^default '; do sleep 0.1; done"

// DiagnoseHost observes the configured installation. It deliberately never
// calls Prepare, defaultRootPool (a lazy creator), EnsureTrustedHost, or a
// reconciler. Unknown ownership prevents execution even inside the trusted host.
func (r *Runtime) DiagnoseHost(ctx context.Context, storage BtrfsLoopPoolSpec) (diagnostics.Report, error) {
	report := diagnostics.Report{}
	for _, name := range diagnostics.CheckNames() {
		report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.Skipped, Summary: "Prerequisite check did not pass", Action: "Resolve the preceding failed checks, then run haco doctor again"})
	}
	check := func(index int, summary, failure, action string, probe func(context.Context) bool) bool {
		budget := 5 * time.Second
		if index == 5 {
			budget = 10 * time.Second
		}
		probeCtx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		ok := probeCtx.Err() == nil && probe(probeCtx) && probeCtx.Err() == nil
		report.Checks[index].Status = diagnostics.Failed
		report.Checks[index].Summary = failure
		report.Checks[index].Action = action
		if ok {
			report.Checks[index].Status = diagnostics.OK
			report.Checks[index].Summary = summary
			report.Checks[index].Action = ""
		}
		return ok
	}
	readJSON := func(ctx context.Context, target any, args ...string) bool {
		result, err := r.runner.Run(ctx, "incus", args...)
		return err == nil && result.ExitCode == 0 && !result.StdoutTruncated && json.Unmarshal([]byte(result.Stdout), target) == nil
	}
	if !check(0, "Incus API is available with trusted management access",
		"Cannot verify Incus API availability and trusted controller access",
		"Check incus.service on the Physical Host; rerun the current installer if setup is incomplete", func(ctx context.Context) bool {
			var api struct {
				Version string `json:"api_version"`
				Auth    string `json:"auth"`
			}
			return readJSON(ctx, &api, "query", "/1.0") && api.Version == "1.0" && api.Auth == "trusted"
		}) {
		return report, nil
	}
	storageSource := ""
	storageOK := check(1, "Configured Btrfs pool and mount policy match",
		"Configured Btrfs pool is unavailable or its driver, state or mount policy differs",
		"Inspect the Incus pool configuration; use haco setup for owned resources and never reformat to repair this check", func(ctx context.Context) bool {
			var pools []struct {
				Name   string            `json:"name"`
				Driver string            `json:"driver"`
				Status string            `json:"status"`
				Config map[string]string `json:"config"`
			}
			if storage.Name == "" || storage.MountOptions == "" || !readJSON(ctx, &pools, "storage", "list", "--project", sandboxResourceProject, "--format", "json") {
				return false
			}
			matches, valid := 0, false
			for _, pool := range pools {
				if pool.Name == storage.Name {
					matches++
					storageSource = pool.Config["source"]
					valid = pool.Driver == "btrfs" && pool.Status == "Created" && pool.Config["btrfs.mount_options"] == storage.MountOptions
				}
			}
			return matches == 1 && valid
		})
	if storageOK {
		matches, known := false, false
		ok := check(2, "Live Incus-owned Btrfs mount applies compress=zstd:3,noatime,nodiscard",
			"Cannot verify the live Incus-owned Btrfs mount and backing identity",
			"Inspect Incus storage and its live mount; do not attach loop devices or reformat to repair diagnostics", func(ctx context.Context) bool {
				matches, known = r.inspectLiveStorage(ctx, storage.Name, storageSource)
				return matches
			})
		if !ok && known && !matches && ctx.Err() == nil {
			report.Checks[2].Status = diagnostics.Pending
			report.Checks[2].Summary = "Configured mount policy matches, but live Btrfs policy differs; application is pending"
			report.Checks[2].Action = "Keep work intact and arrange an Incus-owned pool remount during maintenance; then rerun haco doctor"
		}
	}
	hostOK := check(3, "Owned trusted host is running with its explicit root, NIC and controller endpoint",
		"Cannot verify trusted-host ownership, running state or its explicit devices and controller endpoint",
		"Use ordinary WSL entry for a stopped owned host; for a configuration mismatch inspect controller diagnostics", func(ctx context.Context) bool {
			state, err := r.readTrustedHostNetworkState(ctx)
			if err != nil || state.Status != "Running" || len(state.Profiles) != 0 || !verifyOnlyTrustedHostNIC(state.Devices, trustedHostNetwork) || !verifyOnlyTrustedHostNIC(state.ExpandedDevices, trustedHostNetwork) {
				return false
			}
			root := map[string]string{"type": "disk", "path": "/", "pool": storage.Name}
			proxy := map[string]string{"type": "proxy", "bind": "instance", "listen": "unix:" + trustedHostControlSocket, "connect": "unix:" + defaultPhysicalHostControlSocket, "mode": "0600", "uid": "0", "gid": "0"}
			return reflect.DeepEqual(state.Devices["root"], root) && reflect.DeepEqual(state.ExpandedDevices["root"], root) && reflect.DeepEqual(state.Devices[trustedHostControlDevice], proxy) && reflect.DeepEqual(state.ExpandedDevices[trustedHostControlDevice], proxy) && state.Config[trustedHostControlEnvKey] == trustedHostControlSocket && state.Config["environment.HACO_CLIENT_MODE"] == "controller"
		})
	networkOK := check(4, "Owned trusted bridge has the required DNS, DHCP, NAT and firewall configuration",
		"Cannot verify trusted-bridge ownership or its DNS, DHCP, NAT and firewall configuration",
		"Inspect the owned bridge and current installer diagnostics; do not disable Environment firewall rules", func(ctx context.Context) bool {
			var networks []trustedNetwork
			if !readJSON(ctx, &networks, "network", "list", "--project", sandboxResourceProject, "--format", "json") {
				return false
			}
			matches, valid := 0, false
			for _, network := range networks {
				if network.Name == trustedHostNetwork {
					matches++
					_, err := verifyTrustedHostNetwork(network, r.project)
					valid = err == nil
				}
			}
			return matches == 1 && valid
		})
	if hostOK && networkOK {
		connectivityExit := 0
		startupReady := false
		ok := check(5, "Trusted-host IPv4 DNS, default route and HTTPS to github.com succeed",
			"Trusted-host DNS, default route or HTTPS probe failed or timed out",
			"Check Physical Host DNS, routing and firewall state; rerun haco doctor after restoring connectivity", func(ctx context.Context) bool {
				startup, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project, "--", "env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "timeout", "5", "/bin/sh", "-ec", trustedNetworkStartupProbe)
				if err != nil || startup.ExitCode != 0 || ctx.Err() != nil {
					return false
				}
				startupReady = true
				// Fixed probes only. No caller data is interpolated or sent to the
				// external service, and raw guest output never enters the report.
				result, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project, "--", "env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "timeout", "4", "/bin/sh", "-ec",
					"getent ahostsv4 github.com >/dev/null || exit 21; ip -4 route show default | grep -q '^default ' || exit 22; curl -q -4 -f -sS --connect-timeout 2 --max-time 3 -o /dev/null https://github.com || exit 23")
				connectivityExit = result.ExitCode
				return err == nil && result.ExitCode == 0
			})
		if !ok {
			if !startupReady {
				report.Checks[5].Summary = "Trusted-host DNS service and default IPv4 route did not become ready"
				report.Checks[5].Action = "Allow Host network startup, then rerun haco doctor; inspect guest DNS and DHCP services if this persists"
				return report, nil
			}
			// Only fixed exit markers identify a completed failure stage. A
			// timeout/transport error stays unknown; never infer from guest text.
			switch connectivityExit {
			case 21:
				report.Checks[5].Summary = "Trusted-host IPv4 DNS lookup for github.com failed"
				report.Checks[5].Action = "Check Physical Host DNS and the owned bridge DNS service, then run haco doctor again"
			case 22:
				report.Checks[5].Summary = "Trusted-host default IPv4 route is unavailable"
				report.Checks[5].Action = "Check the owned bridge DHCP service and trusted-host NIC, then run haco doctor again"
			case 23:
				report.Checks[5].Summary = "Trusted-host HTTPS to github.com failed after DNS and route checks passed"
				report.Checks[5].Action = "Check Physical Host HTTPS reachability and forwarding for the owned bridge; keep Environment direct traffic denied"
			}
		}
	}
	return report, nil
}
