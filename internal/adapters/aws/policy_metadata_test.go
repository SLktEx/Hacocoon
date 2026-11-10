package aws

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPolicyDescriptorMatchesS3RequestBuildersWithoutHostAccess(t *testing.T) {
	provider := &Provider{Host: func(context.Context, string, []byte) ([]byte, error) {
		t.Fatal("discovery accessed Host authentication")
		return nil, nil
	}}
	descriptor := provider.PolicyDescriptor()
	if !reflect.DeepEqual(descriptor, (*Provider)(nil).PolicyDescriptor()) || descriptor.Capability != provider.Capability() || len(descriptor.Actions) != 2 {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	spec := ListSpec{Environment: "dev", Profile: "default"}
	identity := identity{Account: "123456789012", Principal: "arn:aws:sts::123456789012:assumed-role/Developer/session", Region: "ap-northeast-1"}
	requests := map[string]core.CapabilityRequest{
		ListAction: request(spec, "example-bucket", "project/", identity),
		GetAction:  getRequest(spec, "example-bucket", "project/config.json", identity),
	}
	resources := map[string]string{ListAction: "arn:aws:s3:::example-bucket", GetAction: "arn:aws:s3:::example-bucket/project/config.json"}
	for _, action := range descriptor.Actions {
		t.Run(action.Action, func(t *testing.T) {
			request, ok := requests[action.Action]
			if !ok || request.Capability != descriptor.Capability || request.Resource != resources[action.Action] {
				t.Fatalf("metadata does not match generated S3 authority: %#v", request)
			}
			var names, keys []string
			for _, attribute := range action.Attributes {
				names = append(names, attribute.Name)
				if len(attribute.Values) != 0 && !slices.Contains(attribute.Values, request.Attributes[attribute.Name]) {
					t.Fatalf("request value missing from %s enum: %q", attribute.Name, request.Attributes[attribute.Name])
				}
			}
			for key := range request.Attributes {
				keys = append(keys, key)
			}
			sort.Strings(names)
			sort.Strings(keys)
			if !reflect.DeepEqual(names, keys) {
				t.Fatalf("metadata attributes %v differ from request attributes %v", names, keys)
			}
		})
	}
}

func TestPolicyDescriptorReturnsIndependentS3Metadata(t *testing.T) {
	first := (*Provider)(nil).PolicyDescriptor()
	for i := range first.Actions {
		first.Actions[i].Notes[0] = "changed"
		for j := range first.Actions[i].Attributes {
			for k := range first.Actions[i].Attributes[j].Values {
				first.Actions[i].Attributes[j].Values[k] = "changed"
			}
		}
	}
	second := (*Provider)(nil).PolicyDescriptor()
	for _, action := range second.Actions {
		if action.Notes[0] == "changed" {
			t.Fatal("descriptor calls share notes")
		}
		for _, attribute := range action.Attributes {
			if slices.Contains(attribute.Values, "changed") {
				t.Fatal("descriptor calls share attribute values")
			}
		}
	}
}
