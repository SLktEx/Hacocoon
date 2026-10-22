package dnsproxy

import (
	"context"
	"net/netip"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/network/dns"
)

type metadataDNSRequester struct{ request core.CapabilityRequest }

func (r *metadataDNSRequester) Request(_ context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	r.request = request
	return core.CapabilityResult{AuditComplete: true, ExecutionState: core.CapabilitySucceeded, Output: "[]"}, nil
}

func TestPolicyDescriptorMatchesDNSRequestWithoutResolverAccess(t *testing.T) {
	provider := Provider{Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("discovery consulted a resolver")
		return nil, nil
	})}
	descriptor := provider.PolicyDescriptor()
	if !reflect.DeepEqual(descriptor, (Provider{}).PolicyDescriptor()) || descriptor.Capability != provider.Capability() || len(descriptor.Actions) != 1 {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	captured := &metadataDNSRequester{}
	if _, err := nameresolution.New(captured).Resolve(context.Background(), "dev", "Example.COM."); err != nil {
		t.Fatal(err)
	}
	request, action := captured.request, descriptor.Actions[0]
	if request.Capability != descriptor.Capability || request.Action != action.Action || request.Resource != "example.com" || len(request.Attributes) != 0 || len(action.Attributes) != 0 {
		t.Fatalf("metadata does not match generated DNS authority: %#v, %#v", descriptor, request)
	}
	first := provider.PolicyDescriptor()
	first.Actions[0].Notes[0] = "changed"
	if provider.PolicyDescriptor().Actions[0].Notes[0] == "changed" {
		t.Fatal("descriptor calls share mutable metadata")
	}
}
