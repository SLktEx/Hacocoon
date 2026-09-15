package environment

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// NoFiniteResourceProvider preserves a provider's existing lifecycle while
// making the v0.12 resource-budget boundary explicit. Finite requests are
// rejected before the wrapped provider can perform side effects; an omitted
// budget is resolved to Hacocoon's explicit unlimited effective budget.
type NoFiniteResourceProvider struct {
	Provider
}

func WithoutFiniteResources(provider Provider) Provider {
	return NoFiniteResourceProvider{Provider: provider}
}

func (p NoFiniteResourceProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if p.Provider == nil {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if core.ResourceBudgetHasFinite(resources) {
		return core.EnvironmentRuntime{}, fmt.Errorf("environment provider does not enforce requested finite resource budget: %w", core.ErrUnsupported)
	}
	spec.Resources = resources
	created, err := p.Provider.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	created.Resources = resources
	return created, nil
}

func (p NoFiniteResourceProvider) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	inspector, ok := p.Provider.(InspectorProvider)
	if !ok {
		return core.EnvironmentRuntimeStatus{}, core.ErrUnsupported
	}
	return inspector.InspectEnvironment(ctx, ref)
}
