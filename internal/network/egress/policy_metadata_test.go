package egress

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPolicyDescriptorMatchesProxyRequests(t *testing.T) {
	descriptor := (Provider{}).PolicyDescriptor()
	if descriptor.Capability != (Provider{}).Capability() || len(descriptor.Actions) != 1 {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	action := descriptor.Actions[0]
	if action.Action != ActionConnect || len(action.Attributes) != 2 {
		t.Fatalf("unexpected connect descriptor: %#v", action)
	}
	var names []string
	for _, attribute := range action.Attributes {
		names = append(names, attribute.Name)
		if attribute.Name == "protocol" && !reflect.DeepEqual(attribute.Values, []string{string(core.EgressHTTP), string(core.EgressHTTPS)}) {
			t.Fatalf("protocol values drifted: %v", attribute.Values)
		}
	}
	sort.Strings(names)
	for _, protocol := range []core.EgressProtocol{core.EgressHTTP, core.EgressHTTPS} {
		captured := &fakeCapabilities{}
		_, err := NewBroker(captured).Authorize(context.Background(), core.EgressRequest{Environment: "dev", Host: "Example.COM.", Port: 443, Protocol: protocol})
		if err != nil {
			t.Fatal(err)
		}
		request := captured.request
		var keys []string
		for key := range request.Attributes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if request.Capability != descriptor.Capability || request.Action != action.Action || request.Resource != "example.com" || !reflect.DeepEqual(names, keys) {
			t.Fatalf("metadata does not match generated authority: %#v, %#v", descriptor, request)
		}
	}
}

func TestPolicyDescriptorReturnsIndependentProxyMetadata(t *testing.T) {
	first := (Provider{}).PolicyDescriptor()
	first.Actions[0].Attributes[1].Values[0] = "changed"
	first.Actions[0].Notes[0] = "changed"
	second := (Provider{}).PolicyDescriptor()
	if second.Actions[0].Attributes[1].Values[0] != string(core.EgressHTTP) || second.Actions[0].Notes[0] == "changed" {
		t.Fatal("descriptor calls share mutable metadata")
	}
}
