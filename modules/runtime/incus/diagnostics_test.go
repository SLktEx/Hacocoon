package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/diagnostics"
	"github.com/SLktEx/Hacocoon/internal/host"
)

var diagnosticStorage = BtrfsLoopPoolSpec{Name: "haco-local-default", MountOptions: "compress=zstd:3,noatime,nodiscard"}

func diagnosticHostState() trustedHostNetworkState {
	state := ownedTrustedNetworkState()
	state.Status = "Running"
	state.Devices["root"]["pool"] = diagnosticStorage.Name
	state.Config[trustedHostControlEnvKey] = trustedHostControlSocket
	state.Config["environment.HACO_CLIENT_MODE"] = "controller"
	state.Devices[trustedHostControlDevice] = map[string]string{
		"type": "proxy", "bind": "instance", "listen": "unix:" + trustedHostControlSocket,
		"connect": "unix:" + defaultPhysicalHostControlSocket, "mode": "0600", "uid": "0", "gid": "0",
	}
	return state
}

func diagnosticFixture(t *testing.T, args []string) host.Result {
	t.Helper()
	switch {
	case reflect.DeepEqual(args, []string{"query", "/1.0"}):
		return jsonResult(map[string]string{"api_version": "1.0", "auth": "trusted"})
	case reflect.DeepEqual(args, []string{"storage", "list", "--project", "default", "--format", "json"}):
		return jsonResult([]any{map[string]any{"name": diagnosticStorage.Name, "driver": "btrfs", "status": "Created", "config": map[string]string{"btrfs.mount_options": diagnosticStorage.MountOptions}}})
	case reflect.DeepEqual(args, []string{"query", "/1.0/instances/haco-host?project=hacocoon"}):
		return jsonResult(diagnosticHostState())
	case reflect.DeepEqual(args, []string{"network", "list", "--project", "default", "--format", "json"}):
		return jsonResult([]trustedNetwork{ownedTrustedNetwork()})
	case len(args) == 13 && reflect.DeepEqual(args[:12], []string{"exec", "haco-host", "--project", "hacocoon", "--", "env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "timeout", "4", "/bin/sh", "-ec"}):
		if !strings.Contains(args[12], "https://github.com") || !strings.Contains(args[12], "curl -q ") {
			t.Fatalf("unexpected probe: %v", args)
		}
		return host.Result{}
	default:
		t.Fatalf("diagnosis attempted unexpected/mutating command: incus %v", args)
		return host.Result{}
	}
}

func TestHostDiagnosticsReadOnlyAndBounded(t *testing.T) {
	runner := &fakeRunner{run: func(ctx context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" {
			t.Fatalf("unexpected executable %s", name)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("probe lacks server-side deadline")
		}
		return diagnosticFixture(t, args), nil
	}}
	runtime := New(runner)
	if err := runtime.ConfigureStorageProvider(func(context.Context) (map[string]string, error) {
		t.Fatal("diagnosis invoked lazy storage creation")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	report, err := runtime.DiagnoseHost(context.Background(), diagnosticStorage)
	if err != nil || !report.Healthy() || len(runner.calls) != 5 {
		t.Fatalf("report=%+v err=%v calls=%v", report, err, runner.calls)
	}
}

func TestHostDiagnosticsFailClosedWithoutExecutingUnownedOrMisconfiguredHost(t *testing.T) {
	for _, name := range []string{"unowned", "stopped", "profile", "extra-nic", "wrong-proxy", "wrong-client", "unowned-network"} {
		t.Run(name, func(t *testing.T) {
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "exec" {
					t.Fatal("executed host after failed ownership/configuration check")
				}
				if strings.HasPrefix(args[1], "/1.0/instances/") {
					state := diagnosticHostState()
					switch name {
					case "unowned":
						state.Config[trustedHostRoleKey] = "environment"
					case "stopped":
						state.Status = "Stopped"
					case "profile":
						state.Profiles = []string{"default"}
					case "extra-nic":
						state.Devices["extra"] = trustedHostNIC("incusbr0")
					case "wrong-proxy":
						state.Devices[trustedHostControlDevice]["connect"] = "unix:/var/lib/incus/unix.socket"
					case "wrong-client":
						state.Config["environment.HACO_CLIENT_MODE"] = "local"
					}
					return jsonResult(state), nil
				}
				if args[0] == "network" && name == "unowned-network" {
					network := ownedTrustedNetwork()
					delete(network.Config, environmentNetworkOwnerKey)
					return jsonResult([]trustedNetwork{network}), nil
				}
				return diagnosticFixture(t, args), nil
			}}
			report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
			if err != nil || report.Healthy() || report.Checks[4].Status != diagnostics.Skipped {
				t.Fatalf("report=%+v err=%v", report, err)
			}
		})
	}
}

func TestHostDiagnosticsRejectMalformedInventoriesAndHideProviderOutput(t *testing.T) {
	for _, target := range []string{"runtime", "storage", "trusted_host", "trusted_network", "trusted_connectivity"} {
		t.Run(target, func(t *testing.T) {
			secret := "Bearer provider-secret"
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				match := target == "runtime" && args[1] == "/1.0" ||
					target == "storage" && args[0] == "storage" ||
					target == "trusted_host" && strings.HasPrefix(args[1], "/1.0/instances/") ||
					target == "trusted_network" && args[0] == "network" ||
					target == "trusted_connectivity" && args[0] == "exec"
				if match {
					return host.Result{Stdout: secret, Stderr: secret, StdoutTruncated: true}, errors.New(secret)
				}
				return diagnosticFixture(t, args), nil
			}}
			report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
			if err != nil || report.Healthy() {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			output, _ := json.Marshal(report)
			if strings.Contains(string(output), secret) {
				t.Fatal("raw provider output leaked")
			}
			if target == "runtime" && len(runner.calls) != 1 {
				t.Fatal("continued after unavailable runtime")
			}
		})
	}
}

func TestHostDiagnosticsRejectStorageDriftWithoutRepair(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[0] == "storage" {
			return jsonResult([]any{map[string]any{"name": diagnosticStorage.Name, "driver": "dir", "status": "Created"}}), nil
		}
		return diagnosticFixture(t, args), nil
	}}
	report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
	if err != nil || report.Checks[1].Status != diagnostics.Failed {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestHostDiagnosticsRejectsAmbiguousSuccessfulInventory(t *testing.T) {
	for _, name := range []string{"null-api", "truncated-api", "duplicate-pool", "duplicate-network", "truncated-host"} {
		t.Run(name, func(t *testing.T) {
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				result := diagnosticFixture(t, args)
				switch {
				case name == "null-api" && args[1] == "/1.0":
					result.Stdout = "null"
				case name == "truncated-api" && args[1] == "/1.0":
					result.StdoutTruncated = true
				case name == "duplicate-pool" && args[0] == "storage",
					name == "duplicate-network" && args[0] == "network":
					var items []json.RawMessage
					if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
						t.Fatal(err)
					}
					result = jsonResult(append(items, items[0]))
				case name == "truncated-host" && strings.HasPrefix(args[1], "/1.0/instances/"):
					result.StdoutTruncated = true
				}
				return result, nil
			}}
			report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
			if err != nil || report.Healthy() {
				t.Fatalf("accepted ambiguous inventory: %+v err=%v", report, err)
			}
		})
	}
}
