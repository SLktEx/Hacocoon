package incus

import (
	"context"
	"encoding/csv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) Probe(ctx context.Context) (core.RuntimeCapabilities, error) {
	result, err := r.runner.Run(ctx, "incus", "version")
	if err != nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"incus unavailable"}}, nil
	}
	return core.RuntimeCapabilities{Available: true, Details: []string{strings.TrimSpace(result.Stdout)}}, nil
}

func (r *Runtime) environmentExists(ctx context.Context, ref string) (bool, error) {
	result, err := r.runner.Run(ctx, "incus", "list", ref, "--project", r.project, "--format", "csv", "-c", "n")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == ref {
			return true, nil
		}
	}
	return false, nil
}

func (r *Runtime) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	if err := validateManagedInstanceRef(ref); err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	result, err := r.runner.Run(ctx, "incus", "list", ref, "--project", r.project, "--format", "csv", "-c", "ns")
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	if result.ExitCode != 0 || result.StdoutTruncated {
		return core.EnvironmentRuntimeStatus{}, core.ErrRuntimeUnavailable
	}
	states := map[string]core.EnvironmentState{
		"RUNNING": core.EnvironmentRunning,
		"STOPPED": core.EnvironmentStopped,
	}
	// Incus name filtering can also return prefixed names (dev and dev-copy).
	// A state without the exact instance name cannot identify this Environment.
	reader := csv.NewReader(strings.NewReader(result.Stdout))
	reader.FieldsPerRecord = 2
	rows, err := reader.ReadAll()
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, core.ErrRuntimeUnavailable
	}
	state := core.EnvironmentUnknown
	found := false
	for _, row := range rows {
		if row[0] != ref {
			continue
		}
		if found {
			return core.EnvironmentRuntimeStatus{}, core.ErrRuntimeUnavailable
		}
		found = true
		if mapped, ok := states[strings.ToUpper(strings.TrimSpace(row[1]))]; ok {
			state = mapped
		}
	}
	return core.EnvironmentRuntimeStatus{State: state}, nil
}

func (r *Runtime) Inspect(ctx context.Context, ref string) (core.RuntimeState, error) {
	status, err := r.InspectEnvironment(ctx, ref)
	if err != nil {
		return core.RuntimeState{}, err
	}
	observed := core.ObservedUnknown
	switch status.State {
	case core.EnvironmentRunning:
		observed = core.ObservedRunning
	case core.EnvironmentStopped:
		observed = core.ObservedStopped
	}
	return core.RuntimeState{Observed: observed}, nil
}
