package incus

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func sandboxProfileResult() host.Result {
	proxy := "http://10.200.0.1:18080"
	return host.Result{Stdout: fmt.Sprintf(`{"config":{"environment.HTTP_PROXY":%q,"environment.HTTPS_PROXY":%q,"environment.NO_PROXY":"127.0.0.1,localhost,::1","environment.http_proxy":%q,"environment.https_proxy":%q,"environment.no_proxy":"127.0.0.1,localhost,::1"},"devices":{"eth0":{"type":"nic","name":"eth0","network":"haco-sandbox0","security.acls":"haco-sandbox-egress","security.acls.default.ingress.action":"reject","security.acls.default.egress.action":"reject","security.acls.default.ingress.logged":"true","security.acls.default.egress.logged":"true","security.ipv4_filtering":"true","security.ipv6_filtering":"true","security.mac_filtering":"true","security.port_isolation":"true"}}}`, proxy, proxy, proxy, proxy)}
}

func managedACLResult() host.Result {
	return host.Result{Stdout: "config: {}\ndescription: \"\"\negress:\n- action: allow\n  state: enabled\n  description: " + sandboxProxyRule + "\n  destination: 10.200.0.1/32\n  protocol: tcp\n  destination_port: \"18080\"\ningress: []\nname: " + sandboxEgressACL + "\n"}
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
	runtime := New(runner)
	if err := runtime.ensureSandboxNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}

	seenACL := false
	seenProfile := false
	seenDNS := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "network acl show "+sandboxEgressACL) {
			seenACL = true
		}
		if strings.Contains(joined, "profile show "+sandboxProfile) {
			seenProfile = true
		}
		if strings.Contains(joined, "network get "+sandboxNetwork+" raw.dnsmasq") {
			seenDNS = true
		}
	}
	if !seenACL || !seenProfile || !seenDNS {
		t.Fatalf("sandbox network verification incomplete: %#v", runner.calls)
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

func TestEnsureSandboxNetworkRejectsACLRulesOutsideProxyTransport(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL {
			return host.Result{Stdout: "egress:\n- action: allow\n  destination: 0.0.0.0/0\n  protocol: tcp\n  destination_port: \"443\"\n  state: enabled\n  description: unmanaged\ningress: []\n"}, nil
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

func TestEnsureSandboxNetworkRejectsUnmanagedDNSConfiguration(t *testing.T) {
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

func TestEnsureSandboxNetworkCreatesMissingResources(t *testing.T) {
	missingNetwork := true
	missingACL := true
	missingProfile := true
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 3 && args[0] == "network" && args[1] == "show" && args[2] == sandboxNetwork && missingNetwork {
			missingNetwork = false
			return host.Result{}, errors.New("missing")
		}
		if len(args) >= 4 && args[0] == "network" && args[1] == "acl" && args[2] == "show" && args[3] == sandboxEgressACL && missingACL {
			missingACL = false
			return host.Result{}, errors.New("missing")
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
		"raw.dnsmasq=port=0",
		"network acl create " + sandboxEgressACL,
		"network acl rule add " + sandboxEgressACL + " egress",
		"destination=10.200.0.1/32",
		"destination_port=18080",
		"profile create " + sandboxProfile,
		"profile device add " + sandboxProfile + " eth0 nic",
		"profile set " + sandboxProfile,
	}
	for _, want := range wantFragments {
		found := false
		for _, call := range runner.calls {
			if strings.Contains(strings.Join(call.args, " "), want) {
				found = true
				break
			}
		if !found {
			t.Fatalf("missing call containing %q: %#v", want, runner.calls)
		}
	}
}

func TestManagedProxyACLRejectsAdditionalRule(t *testing.T) {
	raw := "egress:\n- action: allow\n  state: enabled\n  description: " + sandboxProxyRule + "\n  destination: 10.200.0.1/32\n  protocol: tcp\n  destination_port: \"18080\"\n- action: allow\n  state: enabled\n  description: extra\n  destination: 10.200.0.2/32\n  protocol: tcp\n  destination_port: \"18080\"\ningress: []\n"
	if managedProxyACL(raw, netip.MustParseAddr("10.200.0.1")) {
		t.Fatal("ACL with an additional egress rule must fail closed")
	}
}
