package networkrelay

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"time"
)

type RuleSpec struct {
	Environment string              `json:"environment"`
	Connection  Spec                `json:"connection"`
	Decision    core.PolicyDecision `json:"decision"`
	Scope       string              `json:"scope"`
	ExpiresAt   time.Time           `json:"expires_at"`
}
type PolicyEditor interface {
	Configuration
	Replace(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error)
}

// AddRule resolves the exact destination before saving a bounded administrator
// rule. It does not turn a pending guest request into an approval.
func (s *Service) AddRule(ctx context.Context, configuration PolicyEditor, spec RuleSpec) (capability.PolicyRule, error) {
	if validateSpec(spec.Connection) != nil || !validName(spec.Environment) ||
		(spec.Decision != core.PolicyAllow && spec.Decision != core.PolicyDeny && spec.Decision != core.PolicyRequireApproval) ||
		(spec.Scope != "instance" && spec.Scope != "environment" && spec.Scope != "global") ||
		!spec.ExpiresAt.After(time.Now()) || spec.ExpiresAt.After(time.Now().Add(31*24*time.Hour)) {
		return capability.PolicyRule{}, core.ErrInvalidArgument
	}
	snapshot, err := configuration.Snapshot(ctx)
	if err != nil {
		return capability.PolicyRule{}, err
	}
	instance, err := s.Authority.CurrentInstance(ctx, spec.Environment)
	if err != nil {
		return capability.PolicyRule{}, err
	}
	source := Source{spec.Environment, instance}
	target, err := s.Targets.Resolve(ctx, source, spec.Connection)
	if err != nil {
		return capability.PolicyRule{}, err
	}
	request, err := requestFor(source, target, spec.Connection.DurationSeconds)
	if err != nil {
		return capability.PolicyRule{}, err
	}
	rule := capability.PolicyRule{Capability: Capability, Action: Action, Environment: source.Environment, EnvironmentInstance: instance, Resource: target.Name, Attributes: request.Attributes, Decision: spec.Decision, ExpiresAt: &spec.ExpiresAt}
	if spec.Scope != "instance" {
		rule.EnvironmentInstance = ""
	}
	if spec.Scope == "global" {
		rule.Environment = "*"
	}
	var policy capability.PolicyFile
	if json.Unmarshal(snapshot.Policy, &policy) != nil {
		return capability.PolicyRule{}, core.ErrIncompatibleState
	}
	policy.Rules = append(policy.Rules, rule)
	snapshot.Policy, err = json.Marshal(policy)
	if err != nil {
		return capability.PolicyRule{}, err
	}
	_, err = configuration.Replace(ctx, snapshot)
	return rule, err
}
func ValidateSpec(s Spec) error { return validateSpec(s) }
