package egress

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeCapabilities struct {
	request core.CapabilityRequest
	result  core.CapabilityResult
	err     error
}

func (f *fakeCapabilities) Request(_ context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	f.request = request
	return f.result, f.err
}

func TestCanonicalHostNormalizesDNSNameAndRejectsIPs(t *testing.T) {
	for input, want := range map[string]string{
		"Example.COM":  "example.com",
		"api.example.com.": "api.example.com",
	} {
		got, err := CanonicalHost(input)
		if err != nil || got != want {
			t.Fatalf("CanonicalHost(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"127.0.0.1", "10.0.0.1", "[::1]", "example.com:443", "exa_mple.com", " example.com", "example.com/path"} {
		if _, err := CanonicalHost(input); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("CanonicalHost(%q) error = %v, want ErrInvalidArgument", input, err)
		}
	}
}

func TestBrokerAuthorizesCanonicalHostnameScope(t *testing.T) {
	capabilities := &fakeCapabilities{result: core.CapabilityResult{RequestID: "req-1"}}
	broker := NewBroker(capabilities)
	grant, err := broker.Authorize(context.Background(), core.EgressRequest{
		Environment: "env-a",
		Host:        "GitHub.COM.",
		Port:        443,
		Protocol:    core.EgressHTTPS,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest := core.CapabilityRequest{
		Capability: Capability,
		Action: ActionConnect,
		Resource: "github.com",
		Environment: "env-a",
		Attributes: map[string]string{"protocol": "https", "port": "443"},
	}
	if !reflect.DeepEqual(capabilities.request, wantRequest) {
		t.Fatalf("request = %#v, want %#v", capabilities.request, wantRequest)
	}
	if grant.RequestID != "req-1" || grant.Host != "github.com" || grant.Environment != "env-a" || grant.Port != 443 || grant.Protocol != core.EgressHTTPS {
		t.Fatalf("grant = %#v", grant)
	}
}

func TestBrokerDoesNotIssueGrantWhenCapabilityFails(t *testing.T) {
	capabilities := &fakeCapabilities{err: core.ErrPolicyDenied}
	_, err := NewBroker(capabilities).Authorize(context.Background(), core.EgressRequest{Environment: "env-a", Host: "example.com", Port: 443, Protocol: core.EgressHTTPS})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v, want ErrPolicyDenied", err)
	}
}

func TestProviderRequiresCanonicalExactAuthorityFields(t *testing.T) {
	provider := Provider{}
	valid := core.CapabilityRequest{
		Capability: Capability,
		Action: ActionConnect,
		Resource: "example.com",
		Environment: "env-a",
		Attributes: map[string]string{"protocol": "https", "port": "443"},
	}
	result, err := provider.Execute(context.Background(), valid)
	if err != nil || result.Provider == "" {
		t.Fatalf("Execute(valid) = %#v, %v", result, err)
	}

	invalid := valid
	invalid.Resource = "Example.COM"
	if _, err := provider.Execute(context.Background(), invalid); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("Execute(noncanonical) error = %v, want ErrInvalidArgument", err)
	}
}
