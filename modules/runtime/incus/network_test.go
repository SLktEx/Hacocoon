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
		iifname "haco*" ip daddr 169.254.254.1 tcp dport 18080 accept
		iifname "haco*" drop
	}
	chain forward {
		type filter hook forward priority -200; policy accept;
		iifname "haco*" drop
		oifname "haco*" drop
	}
}`}
}

func managedRoutedSourceGuardResult(table string) host.Result {
	suffix := strings.TrimPrefix(table, sandboxRoutedGuardPrefix)
	iface := sandboxRoutedHostPrefix + suffix
	return host.Result{Stdout: "table inet " + table + " {\n\tchain prerouting {\n\t\ttype filter hook prerouting priority raw; policy accept;\n\t\tiifname \"" + iface + "\" ip saddr != 198.18.0.1 drop\n\t}\n}"}
}

// sandboxNetworkResult is the common fake substrate used by Incus provider
// tests. It deliberately supports both the routed host authority and the
// temporary legacy shared-bridge helpers while the latter are being removed.
func sandboxNetworkResult(args []string) (host.Result, bool) {
	// Routed proxy address inspection.
	if len(args) >= 4 && args[0] == "-o" && args[1] == "-4" && args[2] == "address" && args[3] == "show" {
		return host.Result{Stdout: "1: lo    inet " + sandboxRoutedProxyIPv4 + "/32 scope host lo\n"}, true
	}
	// Routed host route reservation and per-veth route verification.
	if len(args) == 3 && args[0] == "-4" && args[1] == "route" && args[2] == "show" {
		return host.Result{}, true
	}
	if len(args) >= 5 && args[0] == "-4" && args[1] == "route" && args[2] == "show" && args[3] == "dev" {
		return host.Result{Stdout: "198.18.0.1 dev " + args[4] + " scope link\n"}, true
	}
	// All nft mutations are privileged; tests model exact Hacocoon tables so
	// post-start verification exercises the fail-closed rules.
	if len(args) >= 7 && args[0] == "-n" && args[1] == "--" && args[2] == "nft" && args[3] == "list" && args[4] == "table" && args[5] == sandboxRoutedFirewallFamily {
		if args[6] == sandboxRoutedFirewallTable {
			return managedRoutedFirewallResult(), true
		}
		if strings.HasPrefix(args[6], sandboxRoutedGuardPrefix) {
			return managedRoutedSourceGuardResult(args[6]), true
		}
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "/proc/sys/net/ipv4/conf/") && strings.HasSuffix(args[0], "/rp_filter") {
		return host.Result{Stdout: "1\n"}, true
	}
	if len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "get" && args[4] == "eth0" {
		switch args[5] {
		case "ipv4.address":
			return host.Result{Stdout: "198.18.0.1\n"}, true
		case "ipv4.host_address":
			return host.Result{Stdout: sandboxRoutedGatewayIPv4 + "\n"}, true
		}
	}
	if len(args) >= 5 && args[0] == "list" && args[1] == "--project" && args[3] == "--format" && args[4] == "json" {
		return host.Result{Stdout: "[]"}, true
	}

	// Temporary legacy bridge fixtures. No production SandboxProvider uses
	// these after the routed migration; Runtime legacy paths are migrated in a
	// follow-up once the real installer E2E proves the new substrate.
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
