package controlapi

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type workloadSourceResolver interface {
	ResolveEnvironment(context.Context, net.IP) (string, error)
}

// RegisterEnvironmentWorkloads exposes only the OCI workload methods safe for
// an untrusted Environment. The owning Environment is derived from the TCP
// source and persisted runtime state; any Environment value in the request is
// ignored. Registry pulls are deliberately not exposed here because private
// registry authentication belongs to the trusted haco-host boundary.
func RegisterEnvironmentWorkloads(server *control.Server, workloads workloadService, sources workloadSourceResolver) error {
	if server == nil || workloads == nil || sources == nil {
		return control.ErrInvalidArgument
	}
	resolve := func(ctx context.Context) (string, error) {
		environment, err := sources.ResolveEnvironment(ctx, control.PeerIP(ctx))
		if err != nil || strings.TrimSpace(environment) == "" {
			return "", control.NewStatusError("policy_denied", "workload caller is not a managed Environment")
		}
		return environment, nil
	}

	if err := server.Register(MethodWorkloadCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request core.WorkloadSpec
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "name and image are required")
		}
		environment, err := resolve(ctx)
		if err != nil {
			return nil, err
		}
		// Environment callers may only launch from the public Docker Hub OCI
		// remote. Authenticated/private registry acquisition is initiated from
		// haco-host and must not be triggered by an untrusted guest.
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
		environment, err := resolve(ctx)
		if err != nil {
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
		var request WorkloadExecRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Name) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "name and argv are required")
		}
		environment, err := resolve(ctx)
		if err != nil {
			return nil, err
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
		environment, err := resolve(ctx)
		if err != nil {
			return nil, err
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
		environment, err := resolve(ctx)
		if err != nil {
			return nil, err
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
