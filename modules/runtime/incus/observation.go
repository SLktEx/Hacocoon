package incus

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) Probe(ctx context.Context) (core.RuntimeCapabilities, error) {
	output, err := r.readIncusOutput(ctx, "version")
	if err != nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"incus unavailable"}}, nil
	}
	return core.RuntimeCapabilities{Available: true, Details: []string{strings.TrimSpace(output)}}, nil
}

// readIncusOutput is the read-only observation boundary. A partial or failed
// command can never establish absence, ownership or an egress source identity.
func (r *Runtime) readIncusOutput(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	result, err := r.runner.Run(ctx, "incus", args...)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if result.ExitCode != 0 || result.StdoutTruncated {
		return "", core.ErrRuntimeUnavailable
	}
	return result.Stdout, nil
}

func (r *Runtime) readIncusJSON(ctx context.Context, target any, args ...string) error {
	raw, err := r.readIncusOutput(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return core.ErrRuntimeUnavailable
	}
	return nil
}

// Incus name filters can include prefix matches. Only a unique exact row is
// evidence about this instance; malformed output is never an empty inventory.
func (r *Runtime) readInstanceRow(ctx context.Context, ref, columns string) ([]string, error) {
	if err := validateManagedInstanceRef(ref); err != nil {
		return nil, err
	}
	raw, err := r.readIncusOutput(ctx, "list", ref, "--project", r.project, "--format", "csv", "-c", columns)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = len(columns)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, core.ErrRuntimeUnavailable
	}
	var matched []string
	for _, row := range rows {
		if row[0] != ref {
			continue
		}
		if matched != nil {
			return nil, core.ErrRuntimeUnavailable
		}
		matched = row
	}
	return matched, nil
}

func (r *Runtime) environmentExists(ctx context.Context, ref string) (bool, error) {
	row, err := r.readInstanceRow(ctx, ref, "n")
	return row != nil, err
}

func (r *Runtime) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	row, err := r.readInstanceRow(ctx, ref, "ns")
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	status := core.EnvironmentRuntimeStatus{State: core.EnvironmentUnknown, Absent: row == nil}
	if row != nil {
		switch strings.ToUpper(strings.TrimSpace(row[1])) {
		case "RUNNING":
			status.State = core.EnvironmentRunning
		case "STOPPED":
			status.State = core.EnvironmentStopped
		}
	}
	return status, nil
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
