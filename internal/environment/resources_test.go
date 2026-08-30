package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type resourceTestProvider struct {
	createCalls int
}

func (p *resourceTestProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	p.createCalls++
	return core.EnvironmentRuntime{Ref: "runtime-ref"}, nil
}
func (*resourceTestProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*resourceTestProvider) ShellEnvironment(context.Context, string) error { return nil }
func (*resourceTestProvider) DeleteEnvironment(context.Context, string) error { return nil }

func TestNoFiniteResourceProviderRejectsBeforeInnerCreate(t *testing.T) {
	inner := &resourceTestProvider{}
	provider := WithoutFiniteResources(inner)
	_, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/tmp/work",
		Resources: core.ResourceBudget{
			CPU: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 2},
		},
	})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
	if inner.createCalls != 0 {
		t.Fatalf("inner create calls = %d", inner.createCalls)
	}
}

func TestNoFiniteResourceProviderReturnsExplicitUnlimitedBudget(t *testing.T) {
	inner := &resourceTestProvider{}
	provider := WithoutFiniteResources(inner)
	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/work"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Resources != core.UnlimitedResourceBudget() {
		t.Fatalf("resources = %#v", created.Resources)
	}
	if inner.createCalls != 1 {
		t.Fatalf("inner create calls = %d", inner.createCalls)
	}
}
