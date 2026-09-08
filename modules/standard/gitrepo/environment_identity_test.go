package gitrepo

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type identityEnvironmentStore struct {
	environment core.Environment
	identity    string
}

func (s *identityEnvironmentStore) GetEnvironment(context.Context, string) (core.Environment, error) {
	return s.environment, nil
}
func (s *identityEnvironmentStore) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return s.identity, nil
}

type replacingIdentityCapability struct {
	broker   *Broker
	store    *identityEnvironmentStore
	observed string
}

func (c *replacingIdentityCapability) RequestWithApproval(ctx context.Context, request core.CapabilityRequest, _ func(context.Context, core.ApprovalRequest) (bool, error)) (core.CapabilityResult, error) {
	c.observed = request.EnvironmentInstance
	c.store.identity, _ = core.NewEnvironmentInstanceID()
	return c.broker.Execute(ctx, request)
}
func TestGitPreparedOperationRefusesReplacedEnvironmentIdentity(t *testing.T) {
	original, _ := core.NewEnvironmentInstanceID()
	environments := &identityEnvironmentStore{environment: core.Environment{Name: "dev"}, identity: original}
	broker := NewBroker(nil, environments, "")
	capabilities := &replacingIdentityCapability{broker: broker, store: environments}
	broker.Capabilities = capabilities
	executed := false
	_, err := broker.perform(context.Background(), binding{Environment: environments.environment}, Proposal{Environment: "dev", Repository: "repo", Remote: "https://example.com/owner/repo.git", Ref: "refs/heads/main", Operation: "push"}, func(context.Context) (Response, error) { executed = true; return Response{}, nil })
	if !errors.Is(err, core.ErrCapabilityStale) || executed || capabilities.observed != original {
		t.Fatalf("identity replacement was not refused: %v", err)
	}
}
