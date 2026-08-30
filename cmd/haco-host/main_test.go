package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeControllerClient struct{}

func (fakeControllerClient) Ping(context.Context) (controlapi.PingResponse, error) {
	return controlapi.PingResponse{ProtocolVersion: 1}, nil
}

func (fakeControllerClient) CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	return core.Environment{}, nil
}

func (fakeControllerClient) ListEnvironments(context.Context) ([]core.Environment, error) {
	return nil, nil
}

func (fakeControllerClient) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{}, nil
}

func (fakeControllerClient) ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (fakeControllerClient) OpenEnvironmentShell(context.Context, string) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (fakeControllerClient) DeleteEnvironment(context.Context, string) error {
	return nil
}

func TestDispatchRejectsUnknownTopLevelCommand(t *testing.T) {
	err := dispatch(context.Background(), fakeControllerClient{}, []string{"wat"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseCreateRequestMatchesEnvironmentOptions(t *testing.T) {
	request, err := parseCreateRequest([]string{
		"--read-only",
		"--base", "dev",
		"--cpu", "2",
		"--memory", "4GiB",
		"--pids", "256",
		"--root-size", "20GiB",
		"--workspace", "/work/demo",
		"demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Name != "demo" || request.WorkspacePath != "/work/demo" || request.AccessMode != core.WorkspaceReadOnly || request.Base != "dev" {
		t.Fatalf("request = %#v", request)
	}
	if request.Resources.CPU.Mode != core.ResourceLimitFinite || request.Resources.CPU.Value != 2 {
		t.Fatalf("cpu = %#v", request.Resources.CPU)
	}
	if request.Resources.MemoryBytes.Value != 4<<30 || request.Resources.PIDs.Value != 256 || request.Resources.RootBytes.Value != 20<<30 {
		t.Fatalf("resources = %#v", request.Resources)
	}
}

func TestParseCreateRequestRejectsDuplicateWorkspace(t *testing.T) {
	_, err := parseCreateRequest([]string{"--workspace", "/a", "--workspace", "/b", "demo"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
}
