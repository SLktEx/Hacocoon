package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func sandboxProfileResult() host.Result {
	proxy := "http://" + sandboxRoutedProxyIPv4 + ":18080"
	return host.Result{Stdout: `{"config":{"environment.HTTP_PROXY":"` + proxy + `","environment.HTTPS_PROXY":"` + proxy + `","environment.NO_PROXY":"localhost,127.0.0.1,::1","environment.http_proxy":"` + proxy + `","environment.https_proxy":"` + proxy + `","environment.no_proxy":"localhost,127.0.0.1,::1"},"devices":{"eth0":{"type":"nic","name":"eth0","network":"haco-sandbox0","security.ipv4_filtering":"true","security.ipv6_filtering":"true","security.mac_filtering":"true","security.port_isolation":"true"}}}`}
}

func managedACLResult() host.Result {
	return host.Result{Stdout: "config: {}\ndescription: \"\"\negress:\n- action: allow\n  description: Hacocoon Standard egress proxy\n  destination: " + sandboxRoutedProxyIPv4 + "/32\n  destination_port: \"18080\"\n  protocol: tcp\n  state: enabled\ningress: []\nname: " + sandboxEgressACL + "\n"}
}

func managedRoutedFirewallResult() host.Result {
	return host.Result{Stdout: `table inet hacocoon_sandbox {
	chain input {
		type filter hook input priority -200; policy accept;
		iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept
		iifname "hbr*" drop
	}
	chain forward {
		type filter hook forward priority -200; policy accept;
		iifname "hbr*" drop
		oifname "hbr*" drop
	}
}`}
}

func managedRoutedSourceGuardResult(table string) host.Result {
	iface := environmentBridgeName("haco-demo")
	mac := environmentBridgeMAC("haco-demo")
	return host.Result{Stdout: "table inet " + table + " {\n\tchain prerouting {\n\t\ttype filter hook prerouting priority raw; policy accept;\n\t\tiifname \"" + iface + "\" ether saddr != " + mac + " drop\n\t\tiifname \"" + iface + "\" ip saddr != 10.240.0.0/24 drop\n\t}\n}"}
}

// sandboxNetworkResult is the common fake substrate used by Incus provider
// tests. It supports the Environment-dedicated bridge path plus the legacy
// shared bridge helpers still exercised by migration tests.
func sandboxNetworkResult(args []string) (host.Result, bool) {
	if len(args) >= 4 && args[0] == "-o" && args[1] == "-4" && args[2] == "address" && args[3] == "show" {
		return host.Result{Stdout: "1: lo    inet " + sandboxRoutedProxyIPv4 + "/32 scope host lo\n"}, true
	}
	if len(args) >= 7 && args[0] == "-n" && args[1] == "--" && args[2] == "nft" && args[3] == "list" && args[4] == "table" && args[5] == sandboxRoutedFirewallFamily {
		if args[6] == sandboxRoutedFirewallTable {
			return managedRoutedFirewallResult(), true
		}
		if strings.HasPrefix(args[6], sandboxRoutedGuardPrefix) {
			return managedRoutedSourceGuardResult(args[6]), true
		}
	}

	if len(args) >= 3 && args[0] == "network" && args[1] == "show" && strings.HasPrefix(args[2], sandboxRoutedHostPrefix) {
		return host.Result{Stdout: "managed: true\n"}, true
	}
	if len(args) >= 4 && args[0] == "network" && args[1] == "get" && strings.HasPrefix(args[2], sandboxRoutedHostPrefix) {
		values := map[string]string{
			"ipv4.address":  "10.240.0.1/24\n",
			"ipv4.nat":      "false\n",
			"ipv4.firewall": "false\n",
			"ipv4.routing":  "true\n",
			"ipv6.address":  "none\n",
			"raw.dnsmasq":   "port=0\n",
		}
		return host.Result{Stdout: values[args[3]]}, true
	}
	if len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "get" && args[4] == "eth0" {
		ref := args[3]
		bridge := environmentBridgeName(ref)
		switch args[5] {
		case "network":
			return host.Result{Stdout: bridge + "\n"}, true
		case "hwaddr":
			return host.Result{Stdout: environmentBridgeMAC(ref) + "\n"}, true
		}
	}
	if len(args) >= 5 && args[0] == "list" && args[1] == "--project" && args[3] == "--format" && args[4] == "json" {
		return host.Result{Stdout: "[]"}, true
	}

	if len(args) >= 3 && args[0] == "network" && args[1] == "show" && args[2] == sandboxNetwork {
		return host.Result{}, true
	}
	if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[2] == sandboxNetwork {
		values := map[string]string{
			"ipv4.address":                         "10.200.0.1/24\n",
			"ipv4.nat":                             "true\n",
			"ipv4.firewall":                        "true\n",
			"ipv4.routing":                         "true\n",
			"ipv6.address":                         "none\n",
			"raw.dnsmasq":                          "port=0\n",
			"security.acls":                        sandboxEgressACL + "\n",
			"security.acls.default.ingress.action": "reject\n",
			"security.acls.default.egress.action":  "reject\n",
			"security.acls.default.ingress.logged": "true\n",
			"security.acls.default.egress.logged":  "true\n",
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

func TestEnsureSandboxNetworkStillValidatesLegacySubstrateDuringMigration(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}

	seenBridge := false
	seenRoutedFirewall := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "network show "+sandboxNetwork) {
			seenBridge = true
		}
		if strings.Contains(joined, "nft list table "+sandboxRoutedFirewallFamily+" "+sandboxRoutedFirewallTable) {
			seenRoutedFirewall = true
		}
	}
	if !seenBridge || !seenRoutedFirewall {
		t.Fatalf("migration validation incomplete: bridge=%t routedFirewall=%t calls=%#v", seenBridge, seenRoutedFirewall, runner.calls)
	}
}
