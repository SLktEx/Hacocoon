package incus

import (
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEnvironmentBridgeNameIsDeterministicAndLinuxSafe(t *testing.T) {
	first := environmentBridgeName("haco-alpha")
	again := environmentBridgeName("haco-alpha")
	other := environmentBridgeName("haco-beta")
	if first != again {
		t.Fatalf("bridge name is not deterministic: %q != %q", first, again)
	}
	if first == other {
		t.Fatalf("different refs unexpectedly share bridge %q", first)
	}
	if !strings.HasPrefix(first, sandboxRoutedHostPrefix) {
		t.Fatalf("bridge %q does not use Hacocoon prefix %q", first, sandboxRoutedHostPrefix)
	}
	if len(first) > 15 {
		t.Fatalf("bridge %q exceeds Linux IFNAMSIZ payload limit", first)
	}
}

func TestEnvironmentBridgeMACIsDeterministicLocalUnicast(t *testing.T) {
	first := environmentBridgeMAC("haco-alpha")
	again := environmentBridgeMAC("haco-alpha")
	other := environmentBridgeMAC("haco-beta")
	if first != again || first == other {
		t.Fatalf("unexpected Environment MAC identities: first=%q again=%q other=%q", first, again, other)
	}
	if !strings.HasPrefix(first, "02:") {
		t.Fatalf("Environment MAC %q is not locally administered unicast", first)
	}
}

func TestRoutedSandboxGuardTableIsDeterministic(t *testing.T) {
	first := routedSandboxGuardTable("haco-alpha")
	again := routedSandboxGuardTable("haco-alpha")
	other := routedSandboxGuardTable("haco-beta")
	if first != again || first == other || !strings.HasPrefix(first, sandboxRoutedGuardPrefix) {
		t.Fatalf("unexpected guard names: first=%q again=%q other=%q", first, again, other)
	}
}

func TestVerifyRoutedSandboxFirewall(t *testing.T) {
	managed := `table inet hacocoon_sandbox {
	chain input {
		type filter hook input priority -200; policy accept;
		iifname "hbr*" udp sport 68 udp dport 67 accept
		iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept
		iifname "hbr*" drop
	}
	chain forward {
		type filter hook forward priority -200; policy accept;
		iifname "hbr*" drop
		oifname "hbr*" drop
	}
}`
	if err := verifyRoutedSandboxFirewall(managed); err != nil {
		t.Fatalf("managed Environment bridge firewall rejected: %v", err)
	}
	for _, unsafe := range []string{
		strings.Replace(managed, `oifname "hbr*" drop`, "", 1),
		strings.Replace(managed, `iifname "hbr*" udp sport 68 udp dport 67 accept`, `iifname "hbr*" udp accept`, 1),
	} {
		if err := verifyRoutedSandboxFirewall(unsafe); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("unsafe firewall error = %v, want ErrIncompatibleState\n%s", err, unsafe)
		}
	}
}

func TestVerifyEnvironmentSourceGuard(t *testing.T) {
	iface := environmentBridgeName("haco-alpha")
	mac := environmentBridgeMAC("haco-alpha")
	subnet := "10.240.0.0/24"
	managed := `table inet haco_guard_example {
	chain prerouting {
		type filter hook prerouting priority raw; policy accept;
		iifname "` + iface + `" ether saddr != ` + mac + ` drop
		iifname "` + iface + `" ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept
		iifname "` + iface + `" ip saddr != ` + subnet + ` drop
	}
}`
	if err := verifyRoutedSandboxSourceGuard(managed, iface, subnet); err != nil {
		t.Fatalf("managed Environment source guard rejected: %v", err)
	}
	for _, unsafe := range []string{
		strings.Replace(managed, mac, "02:00:00:00:00:01", 1),
		strings.Replace(managed, subnet, "10.241.0.0/24", 1),
		strings.Replace(managed, "ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept", "ip saddr 0.0.0.0 accept", 1),
		strings.Replace(managed, "drop", "accept", 1),
	} {
		if err := verifyRoutedSandboxSourceGuard(unsafe, iface, subnet); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("unsafe source guard error = %v, want ErrIncompatibleState\n%s", err, unsafe)
		}
	}
}
