package capability

import (
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SavedChoice is an explicit persistent user decision, separate from one-shot
// approval. It never infers global scope from an omitted Environment.
type SavedChoice string

const (
	AllowEnvironment SavedChoice = "allow-environment"
	DenyEnvironment  SavedChoice = "deny-environment"
	AllowGlobal      SavedChoice = "allow-global"
	DenyGlobal       SavedChoice = "deny-global"
	AskEnvironment   SavedChoice = "ask-environment"
	AskGlobal        SavedChoice = "ask-global"
)

func RuleForSavedChoice(request core.CapabilityRequest, choice SavedChoice) (PolicyRule, error) {
	if err := validateRequest(request); err != nil {
		return PolicyRule{}, err
	}
	// A literal observed "*" must not turn into wildcard authority when saved.
	if request.Resource == "" || request.Resource == "*" {
		return PolicyRule{}, core.ErrInvalidArgument
	}
	rule := PolicyRule{EnvironmentInstance: request.EnvironmentInstance, Capability: request.Capability, Action: request.Action, Resource: request.Resource, Environment: request.Environment, Attributes: map[string]string{}}
	for key, value := range request.Attributes {
		if value == "*" {
			return PolicyRule{}, fmt.Errorf("cannot persist wildcard request attribute: %w", core.ErrInvalidArgument)
		}
		rule.Attributes[key] = value
	}
	switch choice {
	case AllowEnvironment, DenyEnvironment, AskEnvironment:
		if strings.TrimSpace(request.Environment) == "" || request.Environment == "*" {
			return PolicyRule{}, core.ErrInvalidArgument
		}
	case AllowGlobal, DenyGlobal, AskGlobal:
		rule.Environment = "*"
		rule.EnvironmentInstance = ""
	default:
		return PolicyRule{}, core.ErrInvalidArgument
	}
	switch choice {
	case AllowEnvironment, AllowGlobal:
		rule.Decision = core.PolicyAllow
	case DenyEnvironment, DenyGlobal:
		rule.Decision = core.PolicyDeny
	case AskEnvironment, AskGlobal:
		rule.Decision = core.PolicyRequireApproval
	}
	if err := validatePolicy(PolicyFile{Rules: []PolicyRule{rule}}); err != nil {
		return PolicyRule{}, err
	}
	return rule, nil
}
