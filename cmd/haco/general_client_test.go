package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

type fakeGeneralController struct {
	bases              []core.BaseInfo
	base               core.BaseInfo
	runSpec            runapp.Spec
	runResult          runapp.Result
	runErr             error
	eventOffset        int64
	capabilityRequest  core.CapabilityRequest
	capabilityResult   core.CapabilityResult
	capabilityErr      error
	requireApproval    bool
	approvalResult     bool
}

func (f *fakeGeneralController) ListBases(context.Context) ([]core.BaseInfo, error) {
	return append([]core.BaseInfo(nil), f.bases...), nil
}

func (f *fakeGeneralController) InspectBase(context.Context, core.BaseName) (core.BaseInfo, error) {
	return f.base, nil
}

func (f *fakeGeneralController) Run(_ context.Context, spec runapp.Spec) (runapp.Result, error) {
	f.runSpec = spec
	return f.runResult, f.runErr
}

func (f *fakeGeneralController) StreamEvents(_ context.Context, offset int64, emit func(eventsapp.Event) error) (int64, error) {
	f.eventOffset = offset
	event := eventsapp.Event{
		Time:       time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		Type:       "requested",
		Capability: "demo",
		Action:     "read",
		Decision:   core.PolicyAllow,
		NextOffset: offset + 10,
	}
	if err := emit(event); err != nil {
		return offset, err
	}
	return event.NextOffset, nil
}

func (f *fakeGeneralController) RequestCapability(
	ctx context.Context,
	request core.CapabilityRequest,
	approve func(context.Context, core.ApprovalRequest) (bool, error),
) (core.CapabilityResult, error) {
	f.capabilityRequest = request
	if f.requireApproval {
		approved, err := approve(ctx, core.ApprovalRequest{CapabilityRequest: request, Reason: "test approval"})
		if err != nil {
			return core.CapabilityResult{}, err
		}
		f.approvalResult = approved
		if !approved {
			return core.CapabilityResult{}, core.ErrApprovalDenied
		}
	}
	return f.capabilityResult, f.capabilityErr
}

func TestHandleGeneralControllerBaseList(t *testing.T) {
	client := &fakeGeneralController{bases: []core.BaseInfo{{Name: "ubuntu", Revision: "rev-1"}}}
	var out bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{"base", "list"}, func() (generalControllerClient, error) {
		return client, nil
	}, &bytes.Buffer{}, &out, &bytes.Buffer{})
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if out.String() != "ubuntu\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestHandleGeneralControllerRunPreservesRecoveryFailure(t *testing.T) {
	client := &fakeGeneralController{
		runResult: runapp.Result{Execution: runapp.ExecutionResult{ExitCode: 7, Stdout: "out\n", Stderr: "err\n"}},
		runErr:    core.ErrRecoveryRequired,
	}
	var stdout, stderr bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{"run", "--workspace", "/work", "--", "false"}, func() (generalControllerClient, error) {
		return client, nil
	}, &bytes.Buffer{}, &stdout, &stderr)
	if !handled || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if client.runSpec.WorkspacePath != "/work" || len(client.runSpec.Argv) != 1 || client.runSpec.Argv[0] != "false" {
		t.Fatalf("run spec = %#v", client.runSpec)
	}
	if stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestHandleGeneralControllerRunPreservesGuestExitAfterCleanup(t *testing.T) {
	client := &fakeGeneralController{
		runResult: runapp.Result{
			Execution: runapp.ExecutionResult{ExitCode: 17, Stderr: "guest failed\n"},
			CleanedUp: true,
		},
		runErr: errors.New("exit status 17"),
	}
	var stdout, stderr bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{"run", "--workspace", "/work", "--", "false"}, func() (generalControllerClient, error) {
		return client, nil
	}, &bytes.Buffer{}, &stdout, &stderr)
	if !handled {
		t.Fatal("run was not handled by the general controller client")
	}
	var exitCoder interface{ ExitCode() int }
	if !errors.As(err, &exitCoder) || exitCoder.ExitCode() != 17 {
		t.Fatalf("error = %v, want command exit 17", err)
	}
	if stderr.String() != "guest failed\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHandleGeneralControllerEventsResumesOffset(t *testing.T) {
	client := &fakeGeneralController{}
	var out bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{"events", "--since-offset", "15"}, func() (generalControllerClient, error) {
		return client, nil
	}, &bytes.Buffer{}, &out, &bytes.Buffer{})
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if client.eventOffset != 15 {
		t.Fatalf("offset = %d", client.eventOffset)
	}
	if !strings.Contains(out.String(), "requested\tdemo\tread\tallow") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestHandleGeneralControllerCapabilityPreservesParameters(t *testing.T) {
	client := &fakeGeneralController{capabilityResult: core.CapabilityResult{Output: "done"}}
	var out bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{
		"capability", "request", "demo", "read",
		"--resource", "resource://one",
		"--environment", "env-one",
		"--param", "format=json",
	}, func() (generalControllerClient, error) {
		return client, nil
	}, &bytes.Buffer{}, &out, &bytes.Buffer{})
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if client.capabilityRequest.Parameters["format"] != "json" || client.capabilityRequest.Resource != "resource://one" || client.capabilityRequest.Environment != "env-one" {
		t.Fatalf("request = %#v", client.capabilityRequest)
	}
	if out.String() != "done\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestHandleGeneralControllerCapabilityApprovalUsesClientTerminal(t *testing.T) {
	client := &fakeGeneralController{
		capabilityResult: core.CapabilityResult{Output: "approved"},
		requireApproval:  true,
	}
	stdin := bytes.NewBufferString("yes\n")
	var stdout, stderr bytes.Buffer
	handled, err := handleGeneralControllerArgs(context.Background(), []string{
		"capability", "request", "demo", "write", "--resource", "sensitive",
	}, func() (generalControllerClient, error) {
		return client, nil
	}, stdin, &stdout, &stderr)
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if !client.approvalResult {
		t.Fatal("client approval response was not returned to controller")
	}
	if !strings.Contains(stderr.String(), "[y/N]") || !strings.Contains(stderr.String(), "sensitive") {
		t.Fatalf("approval prompt = %q", stderr.String())
	}
	if stdout.String() != "approved\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestHandleGeneralControllerIgnoresOtherCommands(t *testing.T) {
	factoryCalls := 0
	handled, err := handleGeneralControllerArgs(context.Background(), []string{"connections", "demo"}, func() (generalControllerClient, error) {
		factoryCalls++
		return &fakeGeneralController{}, nil
	}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if handled || err != nil || factoryCalls != 0 {
		t.Fatalf("handled=%t err=%v factoryCalls=%d", handled, err, factoryCalls)
	}
}
