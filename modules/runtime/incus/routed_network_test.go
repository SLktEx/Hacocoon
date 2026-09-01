package incus

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestRoutedSandboxHostInterfaceIsDeterministicAndLinuxSafe(t *testing.T) {
	first := routedSandboxHostInterface("haco-alpha")
	again := routedSandboxHostInterface("haco-alpha")
	other := routedSandboxHostInterface("haco-beta")

	if first != again {
		t.Fatalf("host interface is not deterministic: %q != %q", first, again)
	}
	if first == other {
		t.Fatalf("different refs unexpectedly share host interface %q", first)
	}
	if !strings.HasPrefix(first, sandboxRoutedHostPrefix) {
		t.Fatalf("host interface %q does not use Hacocoon prefix %q", first, sandboxRoutedHostPrefix)
	}
	if len(first) > 15 {
		t.Fatalf("host interface %q exceeds Linux IFNAMSIZ payload limit", first)
	}
}

func TestRoutedSandboxIPv4PoolBoundaries(t *testing.T) {
	cases := []struct {
		offset uint32
		want   string
	}{
		{offset: 0, want: "198.18.0.0"},
		{offset: 65535, want: "198.18.255.255"},
		{offset: 65536, want: "198.19.0.0"},
		{offset: 131071, want: "198.19.255.255"},
	}
	for _, tc := range cases {
		got := routedSandboxIPv4At(tc.offset)
		if got != tc.want {
			t.Fatalf("routedSandboxIPv4At(%d) = %q, want %q", tc.offset, got, tc.want)
		}
		address, err := netip.ParseAddr(got)
		if err != nil {
			t.Fatalf("parse allocated address %q: %v", got, err)
		}
		if !sandboxRoutedPool.Contains(address) {
			t.Fatalf("allocated address %s is outside %s", address, sandboxRoutedPool)
		}
	}
}

func TestPrefixesOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"198.18.0.0/15", "198.18.1.2/32", true},
		{"198.18.1.2/32", "198.18.0.0/15", true},
		{"198.18.0.0/16", "198.19.0.0/16", false},
		{"10.0.0.0/8", "198.18.0.0/15", false},
	}
	for _, tc := range cases {
		got := prefixesOverlap(netip.MustParsePrefix(tc.a), netip.MustParsePrefix(tc.b))
		if got != tc.want {
			t.Fatalf("prefixesOverlap(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestHasExactRoutedSandboxHostRoute(t *testing.T) {
	address := netip.MustParseAddr("198.18.48.105")
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "iproute2 omits dev after filtering", raw: "198.18.48.105 scope link src 169.254.254.1\n", want: true},
		{name: "explicit host prefix", raw: "198.18.48.105/32 scope link\n", want: true},
		{name: "different host route", raw: "198.18.48.106 scope link\n", want: false},
		{name: "broader route is not accepted", raw: "198.18.0.0/15 scope link\n", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasExactRoutedSandboxHostRoute(tc.raw, address); got != tc.want {
				t.Fatalf("hasExactRoutedSandboxHostRoute(%q, %s) = %v, want %v", tc.raw, address, got, tc.want)
			}
		})
	}
}

func TestVerifyRoutedSandboxFirewall(t *testing.T) {
	managed := `table inet hacocoon_sandbox {
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
}`
	if err := verifyRoutedSandboxFirewall(managed); err != nil {
		t.Fatalf("managed routed firewall rejected: %v", err)
	}

	unsafe := strings.Replace(managed, `oifname "haco*" drop`, "", 1)
	err := verifyRoutedSandboxFirewall(unsafe)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("missing isolation rule error = %v, want ErrIncompatibleState", err)
	}
}
