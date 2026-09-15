package incus

import (
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func managedRoutedFirewallResult() host.Result {
	return host.Result{Stdout: `table inet hacocoon_sandbox {
	chain input {
		type filter hook input priority -200; policy accept;
		iifname "hbr*" ct state established,related accept
		iifname "hbr*" udp sport 68 udp dport 67 accept
		iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept
		iifname "hbr*" drop
	}
	chain forward {
		type filter hook forward priority -200; policy accept;
		iifname "hbr*" ip daddr 255.255.255.255 udp sport 68 udp dport 67 accept
		oifname "hbr*" udp sport 67 udp dport 68 accept
		iifname "hbr*" drop
		oifname "hbr*" drop
	}
}`}
}

func managedRoutedSourceGuardResult(table string) host.Result {
	ref := "haco-demo"
	iface := environmentBridgeName(ref)
	mac := environmentBridgeMAC(ref)
	return host.Result{Stdout: "table inet " + table + " {\n\tchain prerouting {\n\t\ttype filter hook prerouting priority raw; policy accept;\n\t\tiifname \"" + iface + "\" ether saddr != " + mac + " drop\n\t\tiifname \"" + iface + "\" ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept\n\t\tiifname \"" + iface + "\" ip saddr != 10.240.0.0/24 drop\n\t}\n}"}
}

// sandboxNetworkResult is the common fake substrate used by Incus provider
// tests. It supports the Environment-dedicated bridge and source guards.
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
			"ipv4.firewall": "true\n",
			"ipv4.dhcp":     "true\n",
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

	return host.Result{}, false
}
