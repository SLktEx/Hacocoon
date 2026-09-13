package capability

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestOfficialBuildPolicyAllowsOnlyScopedPackageTraffic(t *testing.T) {
	instance, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	policy := NewOfficialBuildPolicy(NewFilePolicyEvaluator(t.TempDir() + "/missing-policy.json"))
	release, err := policy.AcquireOfficialBuild(context.Background(), "build-test", []string{"archive.ubuntu.com"})
	if err != nil {
		t.Fatal(err)
	}
	request := func(capability, action, host string, attributes map[string]string) core.CapabilityRequest {
		return core.CapabilityRequest{
			EnvironmentInstance: instance,
			Environment:         "build-test",
			Capability:          capability,
			Action:              action,
			Resource:            host,
			Attributes:          attributes,
		}
	}
	for _, req := range []core.CapabilityRequest{
		request("network.resolve", "lookup", "archive.ubuntu.com", nil),
		request("network.egress", "connect", "archive.ubuntu.com", map[string]string{"protocol": "http", "port": "80"}),
		request("network.egress", "connect", "archive.ubuntu.com", map[string]string{"protocol": "https", "port": "443"}),
	} {
		got, err := policy.Evaluate(context.Background(), req)
		if err != nil || got.Decision != core.PolicyAllow {
			t.Fatalf("allowed request %#v => %#v, %v", req, got, err)
		}
	}
	for _, req := range []core.CapabilityRequest{
		request("network.resolve", "lookup", "example.com", nil),
		request("network.egress", "connect", "archive.ubuntu.com", map[string]string{"protocol": "tcp", "port": "22"}),
		request("git.push", "write", "archive.ubuntu.com", nil),
	} {
		got, err := policy.Evaluate(context.Background(), req)
		if err != nil || got.Decision != core.PolicyDeny {
			t.Fatalf("denied request %#v => %#v, %v", req, got, err)
		}
	}
	other := request("network.resolve", "lookup", "archive.ubuntu.com", nil)
	other.Environment = "ordinary-env"
	got, err := policy.Evaluate(context.Background(), other)
	if err != nil || got.Decision != core.PolicyDeny {
		t.Fatalf("other Environment => %#v, %v", got, err)
	}
	release()
	got, err = policy.Evaluate(context.Background(), request("network.resolve", "lookup", "archive.ubuntu.com", nil))
	if err != nil || got.Decision != core.PolicyDeny {
		t.Fatalf("released grant => %#v, %v", got, err)
	}
}
