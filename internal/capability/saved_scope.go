package capability

import (
	"context"
	"maps"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SavedScopeProvider declares reusable authority for an already prepared operation.
// Only the trusted provider may wildcard changing attribute values. Clients cannot supply
// this scope, and the original request remains the execution/audit identity.
type SavedScopeProvider interface {
	SavedApprovalScope(context.Context, core.CapabilityRequest) (core.CapabilityRequest, error)
}

func savedApprovalScope(ctx context.Context, provider Provider, request core.CapabilityRequest) (core.CapabilityRequest, error) {
	scope := request
	scope.Attributes = maps.Clone(request.Attributes)
	scope.Parameters = nil
	if declarer, ok := provider.(SavedScopeProvider); ok {
		input := scope
		input.Attributes = maps.Clone(scope.Attributes)
		var err error
		scope, err = declarer.SavedApprovalScope(ctx, input)
		if err != nil {
			return core.CapabilityRequest{}, err
		}
	}
	if scope.Capability != request.Capability || scope.Action != request.Action ||
		scope.Resource != request.Resource || scope.Environment != request.Environment ||
		scope.EnvironmentInstance != request.EnvironmentInstance || len(scope.Parameters) != 0 {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	if len(scope.Attributes) != len(request.Attributes) {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	for key, value := range scope.Attributes {
		original, ok := request.Attributes[key]
		if !ok || original == "*" || (original != value && value != "*") {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
	}
	scope.Attributes = maps.Clone(scope.Attributes)
	if err := validateRequest(scope); err != nil {
		return core.CapabilityRequest{}, err
	}
	return scope, nil
}

// RuleForSavedScope accepts explicit wildcards only in a provider-declared,
// controller-validated scope. General client requests must use RuleForSavedChoice.
func RuleForSavedScope(scope core.CapabilityRequest, choice SavedChoice) (PolicyRule, error) {
	literal := scope
	literal.Attributes = maps.Clone(scope.Attributes)
	for key, value := range literal.Attributes {
		if value == "*" {
			literal.Attributes[key] = "scope-placeholder"
		}
	}
	rule, err := RuleForSavedChoice(literal, choice)
	if err != nil {
		return PolicyRule{}, err
	}
	rule.Attributes = maps.Clone(scope.Attributes)
	return rule, nil
}
