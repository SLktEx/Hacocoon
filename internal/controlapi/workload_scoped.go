package controlapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// RegisterBoundEnvironmentWorkloads exposes only the OCI workload methods safe
// for one untrusted Environment. Environment and Incus Project authority are
// supplied by the Physical Host when it creates the dedicated listener. A
// request-carried Project selector is always rejected rather than ignored, so a
// future wire/schema extension cannot accidentally widen guest authority.
func RegisterBoundEnvironmentWorkloads(server *control.Server, workloads workloadService, environment string) error {
	if server == nil || workloads == nil || strings.TrimSpace(environment) == "" || strings.ContainsAny(environment, "/\\\x00\r\n") {
		return control.ErrInvalidArgument
	}

	if err := server.Register(MethodWorkloadCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		var request core.WorkloadSpec
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "name and image are required")
		}
		if request.Environment != "" && request.Environment != environment {
			return nil, control.NewStatusError("policy_denied", "Environment workload request cannot select another Environment")
		}
		if !strings.HasPrefix(request.Image, "oci-docker:") {
			return nil, control.NewStatusError("policy_denied", "Environment workload launch requires the public oci-docker remote")
		}
		request.Environment = environment
		workload, err := workloads.CreateWorkload(ctx, request)
		if err != nil {
			return nil, translateError(err)
		}
		return workload, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadList, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		items, err := workloads.ListWorkloads(ctx, environment)
		if err != nil {
			return nil, translateError(err)
		}
		return items, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadExec, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		var request WorkloadExecRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "name and argv are required")
		}
		result, runErr := workloads.ExecWorkload(ctx, environment, request.Name, request.Argv)
		if runErr != nil && result.ExitCode <= 0 {
			return nil, translateError(runErr)
		}
		return result, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadStop, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		var request WorkloadNameRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" {
			return nil, control.NewStatusError("invalid_argument", "name is required")
		}
		if err := workloads.StopWorkload(ctx, environment, request.Name); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadDelete, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		var request WorkloadNameRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" {
			return nil, control.NewStatusError("invalid_argument", "name is required")
		}
		if err := workloads.DeleteWorkload(ctx, environment, request.Name); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadPull, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if err := rejectGuestProjectSelector(payload); err != nil {
			return nil, err
		}
		var request WorkloadPullRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "image is required")
		}
		if !strings.HasPrefix(request.Image, "oci-docker:") {
			return nil, control.NewStatusError("policy_denied", "Environment image pull requires the public oci-docker remote")
		}
		if err := workloads.PullWorkloadImage(ctx, request.Image); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	return nil
}

func rejectGuestProjectSelector(payload json.RawMessage) error {
	if len(payload) == 0 || string(payload) == "null" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return control.NewStatusError("invalid_argument", "invalid workload request")
	}
	for key := range fields {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
		switch normalized {
		case "project", "incus_project":
			return control.NewStatusError("policy_denied", "Environment workload request cannot select an Incus Project")
		}
	}
	return nil
}
