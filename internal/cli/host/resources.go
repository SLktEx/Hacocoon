package hostcli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/cli/bytesize"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func setResourceOption(budget *core.ResourceBudget, flag, raw string) error {
	if budget == nil {
		return core.ErrInvalidArgument
	}
	var target *core.ResourceLimit
	var max uint64
	byteSized := false
	switch flag {
	case "--cpu":
		target, max = &budget.CPU, core.MaxCPUResourceValue
	case "--memory":
		target, max, byteSized = &budget.MemoryBytes, core.MaxMemoryResourceBytes, true
	case "--pids":
		target, max = &budget.PIDs, core.MaxPIDResourceValue
	case "--root-size":
		target, max, byteSized = &budget.RootBytes, core.MaxRootDiskResourceBytes, true
	default:
		return fmt.Errorf("unknown resource option %q: %w", flag, core.ErrInvalidArgument)
	}
	if target.Mode != core.ResourceLimitUnspecified {
		return fmt.Errorf("duplicate resource option %s: %w", flag, core.ErrInvalidArgument)
	}
	limit, err := parseResourceLimit(raw, max, byteSized)
	if err != nil {
		return fmt.Errorf("%s %q: %w", flag, raw, err)
	}
	*target = limit
	return nil
}

func parseResourceLimit(raw string, max uint64, byteSized bool) (core.ResourceLimit, error) {
	if raw == "unlimited" {
		return core.ResourceLimit{Mode: core.ResourceLimitUnlimited}, nil
	}
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		return core.ResourceLimit{}, core.ErrInvalidArgument
	}
	if byteSized {
		value, err := bytesize.Parse(raw, max)
		if err != nil {
			return core.ResourceLimit{}, err
		}
		return core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: value}, nil
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return core.ResourceLimit{}, core.ErrInvalidArgument
		}
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 || value > max {
		return core.ResourceLimit{}, core.ErrInvalidArgument
	}
	return core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: value}, nil
}

func resourceLimitText(limit core.ResourceLimit) string {
	if limit.Mode == core.ResourceLimitUnlimited || limit.Mode == core.ResourceLimitUnspecified {
		return "unlimited"
	}
	return strconv.FormatUint(limit.Value, 10)
}
