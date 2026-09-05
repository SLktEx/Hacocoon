package incus

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

// DiagnoseHost observes the configured installation. It deliberately never
// calls Prepare, defaultRootPool (a lazy creator), EnsureTrustedHost, or a
// reconciler. Unknown ownership prevents execution even inside the trusted host.
func (r *Runtime) DiagnoseHost(ctx context.Context, storage BtrfsLoopPoolSpec) (diagnostics.Report, error) {
	report := diagnostics.Report{}
	for _, name := range diagnostics.CheckNames() {
		report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.Skipped, Summary: "Prerequisite check did not pass"})
	}
	check := func(index int, summary string, probe func(context.Context) bool) bool {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ok := probeCtx.Err() == nil && probe(probeCtx) && probeCtx.Err() == nil
		report.Checks[index].Status = diagnostics.Failed
		if ok {
			report.Checks[index].Status = diagnostics.OK
		}
		report.Checks[index].Summary = summary
		return ok
	}
	readJSON := func(ctx context.Context, target any, args ...string) bool {
		result, err := r.runner.Run(ctx, "incus", args...)
		return err == nil && result.ExitCode == 0 && !result.StdoutTruncated && json.Unmarshal([]byte(result.Stdout), target) == nil
	}
	if !check(0, "Incus API is available with trusted management access", func(ctx context.Context) bool {
		var api struct {
			Version string `json:"api_version"`
			Auth    string `json:"auth"`
		}
		return readJSON(ctx, &api, "query", "/1.0") && api.Version == "1.0" && api.Auth == "trusted"
	}) {
		return report, nil
	}
	check(1, "Configured Btrfs pool and mount policy match", func(ctx context.Context) bool {
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
				valid = pool.Driver == "btrfs" && pool.Status == "Created" && pool.Config["btrfs.mount_options"] == storage.MountOptions
			}
		}
		return matches == 1 && valid
	})
	hostOK := check(2, "Owned trusted host is running with its explicit root, NIC and controller endpoint", func(ctx context.Context) bool {
		state, err := r.readTrustedHostNetworkState(ctx)
		if err != nil || state.Status != "Running" || len(state.Profiles) != 0 || !verifyOnlyTrustedHostNIC(state.Devices, trustedHostNetwork) || !verifyOnlyTrustedHostNIC(state.ExpandedDevices, trustedHostNetwork) {
			return false
		}
		root := map[string]string{"type": "disk", "path": "/", "pool": storage.Name}
		proxy := map[string]string{"type": "proxy", "bind": "instance", "listen": "unix:" + trustedHostControlSocket, "connect": "unix:" + defaultPhysicalHostControlSocket, "mode": "0600", "uid": "0", "gid": "0"}
		return reflect.DeepEqual(state.Devices["root"], root) && reflect.DeepEqual(state.ExpandedDevices["root"], root) && reflect.DeepEqual(state.Devices[trustedHostControlDevice], proxy) && reflect.DeepEqual(state.ExpandedDevices[trustedHostControlDevice], proxy) && state.Config[trustedHostControlEnvKey] == trustedHostControlSocket && state.Config["environment.HACO_CLIENT_MODE"] == "controller"
	})
	networkOK := check(3, "Owned trusted bridge has the required DNS, DHCP, NAT and firewall configuration", func(ctx context.Context) bool {
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
		check(4, "Trusted-host IPv4 DNS, default route and HTTPS to github.com succeed", func(ctx context.Context) bool {
			// Fixed probes only. No caller data is interpolated or sent to the
			// external service, and raw guest output never enters the report.
			result, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project, "--", "env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "timeout", "4", "/bin/sh", "-ec",
				"getent ahostsv4 github.com >/dev/null; ip -4 route show default | grep -q '^default '; curl -q -4 -f -sS --connect-timeout 2 --max-time 3 -o /dev/null https://github.com")
			return err == nil && result.ExitCode == 0
		})
	}
	return report, nil
}
