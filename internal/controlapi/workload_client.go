package controlapi

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (c *Client) CreateWorkload(ctx context.Context, request core.WorkloadSpec) (core.Workload, error) {
	var response core.Workload
	err := c.wire.Call(ctx, MethodWorkloadCreate, request, &response)
	return response, err
}

func (c *Client) ListWorkloads(ctx context.Context, environment string) ([]core.Workload, error) {
	var response []core.Workload
	err := c.wire.Call(ctx, MethodWorkloadList, WorkloadListRequest{Environment: environment}, &response)
	return response, err
}

func (c *Client) ExecWorkload(ctx context.Context, environment, name string, argv []string) (core.ExecutionResult, error) {
	var response core.ExecutionResult
	err := c.wire.Call(ctx, MethodWorkloadExec, WorkloadExecRequest{
		Environment: environment,
		Name:        name,
		Argv:        append([]string(nil), argv...),
	}, &response)
	return response, err
}

func (c *Client) StopWorkload(ctx context.Context, environment, name string) error {
	return c.wire.Call(ctx, MethodWorkloadStop, WorkloadNameRequest{Environment: environment, Name: name}, nil)
}

func (c *Client) DeleteWorkload(ctx context.Context, environment, name string) error {
	return c.wire.Call(ctx, MethodWorkloadDelete, WorkloadNameRequest{Environment: environment, Name: name}, nil)
}

func (c *Client) PullWorkloadImage(ctx context.Context, image string) error {
	return c.wire.Call(ctx, MethodWorkloadPull, WorkloadPullRequest{Image: image}, nil)
}
