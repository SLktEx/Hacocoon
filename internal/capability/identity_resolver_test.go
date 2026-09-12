package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type identityResolverFixture struct {
	id       string
	failure  error
	observed core.Environment
}

func (r *identityResolverFixture) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	if r.failure != nil {
		return core.Environment{}, r.failure
	}
	r.observed = core.Environment{Name: name}
	return r.observed, nil
}
func (r *identityResolverFixture) EnvironmentInstance(_ context.Context, expected core.Environment) (string, error) {
	if r.failure != nil {
		return "", r.failure
	}
	if expected.Name != r.observed.Name {
		return "", core.ErrCapabilityStale
	}
	return r.id, nil
}
func TestServiceResolvesIdentityBeforeApprovalAndRefusesReplacement(t *testing.T) {
	first, _ := core.NewEnvironmentInstanceID()
	resolver := &identityResolverFixture{id: first}
	audit := &fakeAudit{}
	provider := &fakeProvider{}
	service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval}}, nil, audit, provider)
	service.ConfigureEnvironmentIdentity(resolver)
	_, err := service.RequestWithDecision(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev"}, func(_ context.Context, prompt core.ApprovalRequest) (ApprovalDecision, error) {
		if prompt.CapabilityRequest.EnvironmentInstance != first {
			t.Fatal("approval lacks resolved identity")
		}
		resolver.id, _ = core.NewEnvironmentInstanceID()
		return ApprovalDecision{Approved: true}, nil
	})
	if !errors.Is(err, core.ErrCapabilityStale) || provider.calls != 0 {
		t.Fatalf("replaced identity executed: %v", err)
	}
	for _, event := range audit.events {
		if event.EnvironmentInstance != first {
			t.Fatal("audit identity changed")
		}
	}
}
func TestServiceRejectsAssertedOrUnavailableIdentity(t *testing.T) {
	first, _ := core.NewEnvironmentInstanceID()
	other, _ := core.NewEnvironmentInstanceID()
	for _, tc := range []struct {
		asserted string
		failure  error
	}{{other, nil}, {"", core.ErrNotFound}} {
		provider := &fakeProvider{}
		service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyAllow}}, nil, &fakeAudit{}, provider)
		service.ConfigureEnvironmentIdentity(&identityResolverFixture{id: first, failure: tc.failure})
		if _, err := service.Request(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Environment: "dev", EnvironmentInstance: tc.asserted}); err == nil || provider.calls != 0 {
			t.Fatal("unproved identity executed")
		}
	}
}
