package networkrelay

import (
	"net/netip"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPolicyDescriptorMatchesRelayRequests(t *testing.T) {
	descriptor := (Provider{}).PolicyDescriptor()
	if descriptor.Capability != (Provider{}).Capability() || len(descriptor.Actions) != 1 || descriptor.Actions[0].Action != Action {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	action := descriptor.Actions[0]
	var names []string
	for _, attribute := range action.Attributes {
		names = append(names, attribute.Name)
		switch attribute.Name {
		case "protocol":
			if !reflect.DeepEqual(attribute.Values, []string{"tcp", "udp"}) {
				t.Fatalf("protocol values drifted: %v", attribute.Values)
			}
		case "duration_seconds":
			if !strings.Contains(attribute.Description, strconv.Itoa(MaxDuration)) || !strings.Contains(attribute.Description, strconv.Itoa(MaxUDPDurations)) {
				t.Fatalf("duration bounds drifted: %s", attribute.Description)
			}
		case "destination_kind":
			if !reflect.DeepEqual(attribute.Values, []string{"environment", "external", "host"}) {
				t.Fatalf("destination kinds drifted: %v", attribute.Values)
			}
		}
	}
	sort.Strings(names)
	source := Source{Environment: "source", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	for _, kind := range []string{"external", "environment", "host"} {
		for _, protocol := range []string{"tcp", "udp"} {
			t.Run(kind+"/"+protocol, func(t *testing.T) {
				target := Target{Kind: kind, Name: "database", Protocol: protocol, Port: 5432, Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
				if kind == "environment" {
					target.Owner = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				}
				if kind == "host" {
					target.Owner = "svc-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				}
				request, err := requestFor(source, target, 30)
				if err != nil {
					t.Fatal(err)
				}
				assertRelayMetadataRequest(t, descriptor.Capability, action.Action, names, request)
				for _, attribute := range action.Attributes {
					if len(attribute.Values) != 0 && !slices.Contains(attribute.Values, request.Attributes[attribute.Name]) {
						t.Fatalf("request value missing from %s enum", attribute.Name)
					}
				}
			})
		}
	}
}

func assertRelayMetadataRequest(t *testing.T, capability, action string, names []string, request core.CapabilityRequest) {
	t.Helper()
	var keys []string
	for key := range request.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if request.Capability != capability || request.Action != action || request.Resource != "database" || !reflect.DeepEqual(names, keys) {
		t.Fatalf("metadata does not match generated relay authority: %v, %#v", names, request)
	}
}

func TestPolicyDescriptorReturnsIndependentRelayMetadata(t *testing.T) {
	first := (Provider{}).PolicyDescriptor()
	first.Actions[0].Attributes[2].Values[0] = "changed"
	first.Actions[0].Notes[0] = "changed"
	second := (Provider{}).PolicyDescriptor()
	if second.Actions[0].Attributes[2].Values[0] != "environment" || second.Actions[0].Notes[0] == "changed" {
		t.Fatal("descriptor calls share mutable metadata")
	}
}
