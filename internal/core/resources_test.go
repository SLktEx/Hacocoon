package core

import (
	"errors"
	"testing"
)

func TestResolveResourceBudgetMakesOmissionExplicitUnlimited(t *testing.T) {
	got, err := ResolveResourceBudget(ResourceBudget{})
	if err != nil {
		t.Fatal(err)
	}
	if got != UnlimitedResourceBudget() {
		t.Fatalf("budget = %#v, want %#v", got, UnlimitedResourceBudget())
	}
}

func TestResolveResourceBudgetPreservesFiniteValues(t *testing.T) {
	input := ResourceBudget{
		CPU:         ResourceLimit{Mode: ResourceLimitFinite, Value: 4},
		MemoryBytes: ResourceLimit{Mode: ResourceLimitFinite, Value: 8 << 30},
		PIDs:        ResourceLimit{Mode: ResourceLimitFinite, Value: 1024},
		RootBytes:   ResourceLimit{Mode: ResourceLimitFinite, Value: 40 << 30},
	}
	got, err := ResolveResourceBudget(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Fatalf("budget = %#v, want %#v", got, input)
	}
}

func TestResolveResourceBudgetRejectsInvalidValues(t *testing.T) {
	tests := []ResourceBudget{
		{CPU: ResourceLimit{Mode: ResourceLimitFinite, Value: 0}},
		{CPU: ResourceLimit{Mode: ResourceLimitFinite, Value: MaxCPUResourceValue + 1}},
		{MemoryBytes: ResourceLimit{Mode: ResourceLimitFinite, Value: MaxMemoryResourceBytes + 1}},
		{PIDs: ResourceLimit{Mode: ResourceLimitFinite, Value: MaxPIDResourceValue + 1}},
		{RootBytes: ResourceLimit{Mode: ResourceLimitFinite, Value: MaxRootDiskResourceBytes + 1}},
		{CPU: ResourceLimit{Mode: ResourceLimitUnlimited, Value: 1}},
		{CPU: ResourceLimit{Mode: ResourceLimitUnspecified, Value: 1}},
		{CPU: ResourceLimit{Mode: "mystery", Value: 1}},
	}
	for i, input := range tests {
		_, err := ResolveResourceBudget(input)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("case %d error = %v", i, err)
		}
	}
}

func TestResourceBudgetHasFinite(t *testing.T) {
	if ResourceBudgetHasFinite(UnlimitedResourceBudget()) {
		t.Fatal("unlimited budget reported finite")
	}
	budget := UnlimitedResourceBudget()
	budget.PIDs = ResourceLimit{Mode: ResourceLimitFinite, Value: 64}
	if !ResourceBudgetHasFinite(budget) {
		t.Fatal("finite PID budget not detected")
	}
}
