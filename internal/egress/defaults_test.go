package egress

import (
	"fmt"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDefaultPackageRepositoryEndpointsAreExactAndImmutable(t *testing.T) {
	want := map[string]bool{}
	for _, host := range []string{"archive.ubuntu.com", "security.ubuntu.com", "ports.ubuntu.com"} {
		want[fmt.Sprintf("%s|%s|%d", host, core.EgressHTTP, 80)] = true
		want[fmt.Sprintf("%s|%s|%d", host, core.EgressHTTPS, 443)] = true
	}

	got := DefaultPackageRepositoryEndpoints()
	if len(got) != len(want) {
		t.Fatalf("unexpected endpoint count: got %d want %d", len(got), len(want))
	}
	for _, endpoint := range got {
		key := fmt.Sprintf("%s|%s|%d", endpoint.Host, endpoint.Protocol, endpoint.Port)
		if !want[key] {
			t.Fatalf("unexpected default package endpoint: %#v", endpoint)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing default package endpoints: %#v", want)
	}

	got[0].Host = "packages.example.com"
	fresh := DefaultPackageRepositoryEndpoints()
	for _, endpoint := range fresh {
		if endpoint.Host == "packages.example.com" {
			t.Fatal("caller mutation changed package endpoint source of truth")
		}
	}
}
