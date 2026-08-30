package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func sandboxProfileResult() host.Result {
	return host.Result{Stdout: `{"config":{"environment.HTTP_PROXY":"http://10.200.0.1:18080","environment.HTTPS_PROXY":"http://10.200.0.1:18080","environment.NO_PROXY":"localhost,127.0.0.1,::1","environment.http_proxy":"http://10.200.0.1:18080","environment.https_proxy":"http://10.200.0.1:18080","environment.no_proxy":"localhost,127.0.0.1,::1"},"devices":{"eth0":{"type":"nic","name":"eth0","network":"haco-sandbox0","security.acls":"haco-sandbox-egress","security.acls.default.ingress.action":"reject","security.acls.default.egress.action":"reject","security.acls.default.ingress.logged":"true","security.acls.default.egress.logged":"true","security.ipv4_filtering":"true","security.ipv6_filtering":"true","security.mac_filtering":"true","security.port_isolation":"true"}}}`}
}

func managedACLResult() host.Result {
	return host.Result{Stdout: "config: {}\ndescription: \"\"\negress:\n- action: allow\n  description: Hacocoon Standard egress proxy\n  destination: 10.200.0.1/32\n  destination_port: \"18080\"\n  protocol: tcp\n  state: enabled\ningress: []\nname: " + sandboxEgressACL + "\n"}
}

func emptyACLResult() host.Result {
	return host.Result{Stdout: "config: {}\ndescription: \"\"\negress: []\ningress: []\nname: " + sandboxEgressACL + "\n"}
}

func sandboxNetworkResult(args []string) (host.Result, bool) {
	if len(args) >= 3 && args[0] == "network" && args[1] == "show" && args[2] == sandboxNetwork {
		return host.Result{}, true
	}
	if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[2] == sandboxNetwork {
		values := map[string]string{
			"ipv4.address":  "10.200.0.1/24\n",
			"ipv4.nat":      "true\n",
			"ipv4.firewall": "true\n",
			"ipv4.routing":  "true\n",
			"ipv6.address":  "none\n",
			"raw.dnsmasq":   "port=0\n",
		}
		return host.Result{Stdout: values[args[3]]}, true
	}
	if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL {
		return managedACLResult(), true
	}
	if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == sandboxProfile {
		return sandboxProfileResult(), true
	}
	return host.Result{}, false
}

func TestEnsureSandboxNetworkAcceptsManagedProxyOnlySubstrate(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}

	seenACL := false
	seenDNS := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "network acl show "+sandboxEgressACL) {
			seenACL = true
		}
		if strings.Contains(joined, "network get "+sandboxNetwork+" raw.dnsmasq") {
			seenDNS = true
		}
	}
	if !seenACL || !seenDNS {
		t.Fatalf("sandbox network verification incomplete: %#v", runner.calls)
	}
}

func TestEnsureSandboxNetworkMigratesLegacyEmptyACLAndDisablesDNS(t *testing.T) {
	dnsDisabled := false
	ruleAdded := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[2] == sandboxNetwork && args[3] == "raw.dnsmasq" {
			if dnsDisabled {
				return host.Result{Stdout: "port=0\n"}, nil
			}
			return host.Result{}, nil
		}
		if len(args) >= 4 && args[0] == "network" && args[1] == "set" && args[2] == sandboxNetwork && args[3] == "raw.dnsmasq=port=0" {
			dnsDisabled = true
			return host.Result{}, nil
		}
		if len(args) >= 5 && args[0] == "network" && args[1] == "acl" && args[2] == "rule" && args[3] == "add" {
			ruleAdded = true
			return host.Result{}, nil
		}
		if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL {
			if ruleAdded {
				return managedACLResult(), nil
			}
			return emptyACLResult(), nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !dnsDisabled || !ruleAdded {
		t.Fatalf("legacy network was not migrated: dns=%t rule=%t calls=%#v", dnsDisabled, ruleAdded, runner.calls)
	}
}

func TestEnsureSandboxNetworkRejectsUnmanagedDNSConfig(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[2] == sandboxNetwork && args[3] == "raw.dnsmasq" {
			return host.Result{Stdout: "server=8.8.8.8\n"}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestEnsureSandboxNetworkRejectsProfileDrift(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == sandboxProfile {
			return host.Result{Stdout: `{"config":{"security.privileged":"true"},"devices":{}}`}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestEnsureSandboxNetworkRejectsBroadACLRules(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL {
			return host.Result{Stdout: "egress:\n- action: allow\n  destination: 0.0.0.0/0\n  destination_port: \"18080\"\n  protocol: tcp\n  state: enabled\ningress: []\n"}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestEnsureSandboxNetworkCreatesMissingResources(t *testing.T) {
	missingNetwork := true
	missingACL := true
	missingProfile := true
	ruleAdded := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 3 && args[0] == "network" && args[1] == "show" && args[2] == sandboxNetwork && missingNetwork {
			missingNetwork = false
			return host.Result{}, errors.New("missing")
		}
		if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL {
			if missingACL {
				missingACL = false
				return host.Result{}, errors.New("missing")
			}
			if ruleAdded {
				return managedACLResult(), nil
			}
			return emptyACLResult(), nil
		}
		if len(args) >= 5 && args[0] == "network" && args[1] == "acl" && args[2] == "rule" && args[3] == "add" {
			ruleAdded = true
			return host.Result{}, nil
		}
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == sandboxProfile && missingProfile {
			missingProfile = false
			return host.Result{}, errors.New("missing")
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}

	wantFragments := []string{
		"network create " + sandboxNetwork,
		"network acl create " + sandboxEgressACL,
		"network acl rule add " + sandboxEgressACL + " egress",
		"profile create " + sandboxProfile,
		"profile device add " + sandboxProfile + " eth0 nic",
	}
	for _, want := range wantFragments {
		found := false
		for _, call := range runner.calls {
			if strings.Contains(strings.Join(call.args, " "), want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing call containing %q: %#v", want, runner.calls)
		}
	}
}
