package core

import "fmt"

type ResourceLimitMode string

const (
	ResourceLimitUnspecified ResourceLimitMode = ""
	ResourceLimitUnlimited   ResourceLimitMode = "unlimited"
	ResourceLimitFinite      ResourceLimitMode = "finite"
)

const (
	MaxCPUResourceValue      uint64 = 4096
	MaxMemoryResourceBytes   uint64 = 1 << 60
	MaxPIDResourceValue      uint64 = 1<<31 - 1
	MaxRootDiskResourceBytes uint64 = 1 << 60
)

type ResourceLimit struct {
	Mode  ResourceLimitMode `json:"mode"`
	Value uint64            `json:"value,omitempty"`
}

type ResourceBudget struct {
	CPU         ResourceLimit `json:"cpu"`
	MemoryBytes ResourceLimit `json:"memory_bytes"`
	PIDs        ResourceLimit `json:"pids"`
	RootBytes   ResourceLimit `json:"root_bytes"`
}

func UnlimitedResourceBudget() ResourceBudget {
	unlimited := ResourceLimit{Mode: ResourceLimitUnlimited}
	return ResourceBudget{
		CPU:         unlimited,
		MemoryBytes: unlimited,
		PIDs:        unlimited,
		RootBytes:   unlimited,
	}
}

func ResolveResourceBudget(input ResourceBudget) (ResourceBudget, error) {
	cpu, err := resolveResourceLimit("cpu", input.CPU, MaxCPUResourceValue)
	if err != nil {
		return ResourceBudget{}, err
	}
	memory, err := resolveResourceLimit("memory", input.MemoryBytes, MaxMemoryResourceBytes)
	if err != nil {
		return ResourceBudget{}, err
	}
	pids, err := resolveResourceLimit("pids", input.PIDs, MaxPIDResourceValue)
	if err != nil {
		return ResourceBudget{}, err
	}
	root, err := resolveResourceLimit("root-size", input.RootBytes, MaxRootDiskResourceBytes)
	if err != nil {
		return ResourceBudget{}, err
	}
	return ResourceBudget{CPU: cpu, MemoryBytes: memory, PIDs: pids, RootBytes: root}, nil
}

func resolveResourceLimit(name string, input ResourceLimit, max uint64) (ResourceLimit, error) {
	switch input.Mode {
	case ResourceLimitUnspecified:
		if input.Value != 0 {
			return ResourceLimit{}, fmt.Errorf("%s resource value without mode: %w", name, ErrInvalidArgument)
		}
		return ResourceLimit{Mode: ResourceLimitUnlimited}, nil
	case ResourceLimitUnlimited:
		if input.Value != 0 {
			return ResourceLimit{}, fmt.Errorf("%s unlimited resource has a numeric value: %w", name, ErrInvalidArgument)
		}
		return input, nil
	case ResourceLimitFinite:
		if input.Value == 0 || input.Value > max {
			return ResourceLimit{}, fmt.Errorf("%s resource value %d is outside 1..%d: %w", name, input.Value, max, ErrInvalidArgument)
		}
		return input, nil
	default:
		return ResourceLimit{}, fmt.Errorf("unknown %s resource mode %q: %w", name, input.Mode, ErrInvalidArgument)
	}
}

func ResourceBudgetHasFinite(budget ResourceBudget) bool {
	return budget.CPU.Mode == ResourceLimitFinite ||
		budget.MemoryBytes.Mode == ResourceLimitFinite ||
		budget.PIDs.Mode == ResourceLimitFinite ||
		budget.RootBytes.Mode == ResourceLimitFinite
}
