package controlapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	MethodWorkloadCreate = "workload.create"
	MethodWorkloadList   = "workload.list"
	MethodWorkloadExec   = "workload.exec"
	MethodWorkloadStop   = "workload.stop"
	MethodWorkloadDelete = "workload.delete"
	MethodWorkloadPull   = "workload.pull"
)

type WorkloadNameRequest struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
}

type WorkloadListRequest struct {
	Environment string `json:"environment"`
}

type WorkloadExecRequest struct {
	Environment string   `json:"environment"`
	Name        string   `json:"name"`
	Argv        []string `json:"argv"`
}

type WorkloadPullRequest struct {
	Image string `json:"image"`
}

type workloadService interface {
	CreateWorkload(context.Context, core.WorkloadSpec) (core.Workload, error)
	ListWorkloads(context.Context, string) ([]core.Workload, error)
	ExecWorkload(context.Context, string, string, []string) (core.ExecutionResult, error)
	StopWorkload(context.Context, string, string) error
	DeleteWorkload(context.Context, string, string) error
	PullWorkloadImage(context.Context, string) error
}

func RegisterWorkloads(server *control.Server, workloads workloadService) error {
	if server == nil || workloads == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodWorkloadCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request core.WorkloadSpec
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid workload create request")
		}
		if strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "environment, name, and image are required")
		}
		workload, err := workloads.CreateWorkload(ctx, request)
		if err != nil {
			return nil, translateError(err)
		}
		return workload, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadList, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request WorkloadListRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" {
			return nil, control.NewStatusError("invalid_argument", "environment is required")
		}
		items, err := workloads.ListWorkloads(ctx, request.Environment)
		if err != nil {
			return nil, translateError(err)
		}
		return items, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadExec, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request WorkloadExecRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.Name) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "environment, name, and argv are required")
		}
		result, err := workloads.ExecWorkload(ctx, request.Environment, request.Name, request.Argv)
		if err != nil && result.ExitCode <= 0 {
			return nil, translateError(err)
		}
		return result, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadStop, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeWorkloadName(payload)
		if err != nil {
			return nil, err
		}
		if err := workloads.StopWorkload(ctx, request.Environment, request.Name); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadDelete, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeWorkloadName(payload)
		if err != nil {
			return nil, err
		}
		if err := workloads.DeleteWorkload(ctx, request.Environment, request.Name); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodWorkloadPull, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request WorkloadPullRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Image) == "" {
			return nil, control.NewStatusError("invalid_argument", "image is required")
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

func decodeWorkloadName(payload json.RawMessage) (WorkloadNameRequest, error) {
	var request WorkloadNameRequest
	if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.Name) == "" {
		return WorkloadNameRequest{}, control.NewStatusError("invalid_argument", "environment and name are required")
	}
	return request, nil
}
