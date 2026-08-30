package controlapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// RegisterBoundEnvironmentWorkloads exposes only the OCI workload methods safe
// for one untrusted Environment. The Environment identity is supplied by the
// Physical Host when it creates this dedicated Unix listener; request payloads
// can never select another Environment. Registry pulls are deliberately not
// exposed here because private registry authentication belongs to haco-host.
func RegisterBoundEnvironmentWorkloads(server *control.Server, workloads workloadService, environment string) error {
	if server == nil || workloads == nil || strings.TrimSpace(environment) == "" || strings.ContainsAny(environment, "/\\\x00\r\n") {
		return control.ErrInvalidArgument
	}

	if err := server.Register(MethodWorkloadCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request core.WorkloadSpec
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "name and image are required")
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
	if err := server.Register(MethodWorkloadList, func(ctx context.Context, _ json.RawMessage) (any, error) {
		items, err := workloads.ListWorkloads(ctx, environment)
		if err != nil {
			return nil, translateError(err)
		}
		return items, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadExec, func(ctx context.Context, payload json.RawMessage) (any, error) {
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
	return nil
}
