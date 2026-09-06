package incus

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func ownedTrustedNetwork() trustedNetwork {
	config := map[string]string{"ipv4.address": "10.70.80.1/24"}
	for key, value := range trustedHostNetworkConfig {
		config[key] = value
	}
	return trustedNetwork{Name: trustedHostNetwork, Type: "bridge", Managed: true, Config: config}
}

func ownedTrustedNetworkState() trustedHostNetworkState {
	devices := map[string]map[string]string{
		"eth0": trustedHostNIC(trustedHostNetwork),
		"root": {"type": "disk", "path": "/", "pool": "default"},
	}
	return trustedHostNetworkState{Name: trustedHostName, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue}, Devices: devices, ExpandedDevices: devices, Profiles: []string{}}
}

func jsonResult(value any) host.Result {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return host.Result{Stdout: string(data)}
}

func trustedHostNetworkFixture(args []string) (host.Result, bool) {
	if len(args) >= 2 && args[0] == "network" && args[1] == "list" {
		return jsonResult([]trustedNetwork{ownedTrustedNetwork()}), true
	}
	if len(args) >= 2 && args[0] == "query" && strings.HasPrefix(args[1], "/1.0/instances/haco-host?") {
		return jsonResult(ownedTrustedNetworkState()), true
	}
	if reflect.DeepEqual(args, []string{"-n", "--", "iptables", "-w", "5", "-S"}) {
		return host.Result{Stdout: "-P INPUT ACCEPT\n-P FORWARD ACCEPT\n-P OUTPUT ACCEPT\n"}, true
	}
	return host.Result{}, false
}

func TestTrustedHostNetworkRejectsUnownedOrUnsafeState(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*trustedNetwork)
	}{
		{"unowned", func(n *trustedNetwork) { delete(n.Config, environmentNetworkOwnerKey) }},
		{"foreign-owner", func(n *trustedNetwork) { n.Config[environmentNetworkOwnerKey] = environmentNetworkOwnerValue }},
		{"unmanaged", func(n *trustedNetwork) { n.Managed = false }},
		{"wrong-type", func(n *trustedNetwork) { n.Type = "physical" }},
		{"dns-override", func(n *trustedNetwork) { n.Config["raw.dnsmasq"] = "address=/#/1.2.3.4" }},
		{"firewall-disabled", func(n *trustedNetwork) { n.Config["ipv4.firewall"] = "false" }},
		{"external-interface", func(n *trustedNetwork) { n.Config["bridge.external_interfaces"] = "eth0" }},
		{"malformed-subnet", func(n *trustedNetwork) { n.Config["ipv4.address"] = "-j ACCEPT" }},
		{"public-subnet", func(n *trustedNetwork) { n.Config["ipv4.address"] = "8.8.8.1/24" }},
		{"environment-attached", func(n *trustedNetwork) { n.UsedBy = []string{"/1.0/instances/haco-env?project=hacocoon"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			network := ownedTrustedNetwork()
			test.change(&network)
			if _, err := verifyTrustedHostNetwork(network, defaultProject); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestTrustedHostNetworkCreationMarksOwnershipAndDoesNotTouchStorage(t *testing.T) {
	exists := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name == "incus" && len(args) > 1 && args[0] == "network" {
			if args[1] == "list" {
				if !exists {
					return host.Result{Stdout: "[]"}, nil
				}
				return jsonResult([]trustedNetwork{ownedTrustedNetwork()}), nil
			}
			if args[1] == "create" {
				if !strings.Contains(strings.Join(args, " "), environmentNetworkOwnerKey+"="+trustedHostNetworkOwner) {
					t.Fatal("ownership missing from create")
				}
				exists = true
				return host.Result{}, errors.New("ambiguous create response")
			}
		}
		if result, ok := trustedHostNetworkFixture(args); ok {
			return result, nil
		}
		t.Fatalf("unexpected command %s %v", name, args)
		return host.Result{}, nil
	}}
	if err := New(runner).ensureTrustedHostNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := len(runner.calls)
	if err := New(runner).ensureTrustedHostNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls[before:] {
		if strings.Contains(strings.Join(call.args, " "), "create") {
			t.Fatal("non-idempotent bridge creation")
		}
	}
}

func TestTrustedHostNetworkInventoryFailureDoesNotCreate(t *testing.T) {
	for _, output := range []string{"", "null", "{broken", "[] trailing"} {
		runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
			if args[0] != "network" || args[1] != "list" {
				t.Fatalf("mutation after invalid inventory: %v", args)
			}
			return host.Result{Stdout: output}, nil
		}}
		if err := New(runner).ensureTrustedHostNetwork(context.Background()); err == nil {
			t.Fatalf("accepted %q", output)
		}
	}
}

func TestTrustedHostForwardingUnderDockerDropIsScopedAndIdempotent(t *testing.T) {
	installed := map[string]bool{}
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "sudo" || len(args) < 6 || args[2] != "iptables" {
			t.Fatalf("unexpected privileged call %s %v", name, args)
		}
		if args[5] == "-S" {
			return host.Result{Stdout: "-P FORWARD DROP\n-N DOCKER-USER\n-A FORWARD -j DOCKER-USER\n"}, nil
		}
		if args[6] != "DOCKER-USER" {
			t.Fatalf("modified unrelated chain: %v", args)
		}
		switch args[5] {
		case "-C":
			if installed[strings.Join(args[7:], " ")] {
				return host.Result{}, nil
			}
			return host.Result{ExitCode: 1}, errors.New("absent")
		case "-I":
			if args[7] != "1" {
				t.Fatal("rule must precede Docker RETURN")
			}
			rule := strings.Join(args[8:], " ")
			if strings.Contains(rule, "hbr") || !strings.Contains(rule, "haco-host0") || !strings.Contains(rule, "10.70.80.0/24") {
				t.Fatalf("broad rule: %v", args)
			}
			if args[8] == "-o" && !strings.Contains(rule, "RELATED,ESTABLISHED") {
				t.Fatalf("unsolicited inbound allowed: %v", args)
			}
			installed[rule] = true
			return host.Result{}, nil
		default:
			t.Fatalf("global policy or unsupported mutation: %v", args)
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	for range 2 {
		if err := runtime.ensureTrustedHostForwarding(context.Background(), netip.MustParsePrefix("10.70.80.0/24")); err != nil {
			t.Fatal(err)
		}
	}
	if len(installed) != 2 {
		t.Fatalf("rules=%v", installed)
	}
	inserts := 0
	for _, call := range runner.calls {
		if len(call.args) > 5 && call.args[5] == "-I" {
			inserts++
		}
	}
	if inserts != 2 {
		t.Fatalf("inserted %d rules", inserts)
	}
}

func TestTrustedHostForwardingFailsClosedWithoutSafeExtensionPoint(t *testing.T) {
	for _, output := range []string{"", "-P FORWARD DROP\n", "-P FORWARD DROP\n-N DOCKER-USER\n", "-P FORWARD nonsense\n"} {
		runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
			if args[len(args)-1] != "-S" {
				t.Fatalf("mutated firewall on uncertain state: %v", args)
			}
			return host.Result{Stdout: output}, nil
		}}
		if err := New(runner).ensureTrustedHostForwarding(context.Background(), netip.MustParsePrefix("10.70.80.0/24")); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestTrustedHostNICMigrationPreservesRootAndResumesAfterProfileFailure(t *testing.T) {
	state := ownedTrustedNetworkState()
	state.Devices = map[string]map[string]string{"root": state.Devices["root"]}
	state.ExpandedDevices = map[string]map[string]string{"root": state.Devices["root"], "eth0": trustedHostNIC("incusbr0")}
	state.Profiles = []string{"default"}
	removeAttempts, stops := 0, 0
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		switch {
		case args[0] == "query":
			return jsonResult(state), nil
		case args[0] == "stop":
			stops++
			return host.Result{}, nil
		case reflect.DeepEqual(args[:3], []string{"config", "device", "override"}):
			state.Devices["eth0"] = trustedHostNIC(trustedHostNetwork)
			state.ExpandedDevices["eth0"] = state.Devices["eth0"]
			return host.Result{}, nil
		case args[0] == "profile" && args[1] == "remove":
			removeAttempts++
			if removeAttempts == 1 {
				return host.Result{}, errors.New("interrupted")
			}
			state.Profiles = []string{}
			return host.Result{}, nil
		default:
			t.Fatalf("unexpected migration mutation %s %v", name, args)
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	if _, err := runtime.reconcileTrustedHostNIC(context.Background(), "RUNNING", "default"); err == nil {
		t.Fatal("ignored profile failure")
	}
	if _, err := runtime.reconcileTrustedHostNIC(context.Background(), "STOPPED", "default"); err != nil {
		t.Fatal(err)
	}
	if stops != 1 || state.Devices["root"]["pool"] != "default" || len(state.Profiles) != 0 {
		t.Fatalf("state=%+v stops=%d", state, stops)
	}
}

func TestTrustedHostNICRejectsUnknownProfilesWithoutMutation(t *testing.T) {
	state := ownedTrustedNetworkState()
	state.Profiles = []string{"custom"}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[0] != "query" {
			t.Fatalf("unmanaged host mutated: %v", args)
		}
		return jsonResult(state), nil
	}}
	if _, err := New(runner).reconcileTrustedHostNIC(context.Background(), "RUNNING", "default"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error=%v", err)
	}
}
