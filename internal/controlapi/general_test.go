package controlapi

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

type fakeBases struct{}

func (fakeBases) ListBases(context.Context) ([]core.BaseInfo, error) {
	return []core.BaseInfo{{Name: "ubuntu", Revision: "rev-1"}}, nil
}

func (fakeBases) InspectBase(_ context.Context, name core.BaseName) (core.BaseInfo, error) {
	if name != "ubuntu" {
		return core.BaseInfo{}, core.ErrNotFound
	}
	return core.BaseInfo{Name: name, Revision: "rev-1"}, nil
}

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, spec runapp.Spec) (runapp.Result, error) {
	if spec.WorkspacePath == "" || len(spec.Argv) == 0 {
		return runapp.Result{}, core.ErrInvalidArgument
	}
	if spec.Argv[0] == "fail" {
		return runapp.Result{
			Environment: "run-failed",
			Execution: runapp.ExecutionResult{ExitCode: 9, Stderr: "failed\n"},
			CleanedUp: false,
		}, core.ErrRecoveryRequired
	}
	return runapp.Result{
		Environment: "run-ok",
		Execution: runapp.ExecutionResult{ExitCode: 0, Stdout: "ok\n"},
		CleanedUp: true,
	}, nil
}

type fakeEvents struct{}

func (fakeEvents) Stream(_ context.Context, offset int64, emit func(eventsapp.Event) error) (int64, error) {
	if offset == 99 {
		return offset, core.ErrInvalidArgument
	}
	events := []eventsapp.Event{
		{Type: "requested", Capability: "demo", Action: "read", NextOffset: 10},
		{Type: "completed", Capability: "demo", Action: "read", NextOffset: 20},
	}
	for _, event := range events {
		if event.NextOffset <= offset {
			continue
		}
		if err := emit(event); err != nil {
			return offset, err
		}
		offset = event.NextOffset
	}
	return offset, nil
}

type fakeCapabilities struct {
	request  core.CapabilityRequest
	approved bool
}

func (f *fakeCapabilities) RequestWithApproval(
	ctx context.Context,
	request core.CapabilityRequest,
	approve func(context.Context, core.ApprovalRequest) (bool, error),
) (core.CapabilityResult, error) {
	f.request = request
	result := core.CapabilityResult{Provider: "demo", Output: "accepted", RequestID: "req-1", ExecutionState: core.CapabilitySucceeded, AuditComplete: true}
	if request.Action == "approve" {
		approved, err := approve(ctx, core.ApprovalRequest{CapabilityRequest: request, Reason: "human approval required"})
		if err != nil {
			return core.CapabilityResult{}, err
		}
		f.approved = approved
		if !approved {
			result.ExecutionState = core.CapabilityNotExecuted
			return result, core.ErrApprovalDenied
		}
	}
	if request.Action == "deny" {
		result.ExecutionState = core.CapabilityNotExecuted
		return result, core.ErrPolicyDenied
	}
	return result, nil
}

func TestGeneralControllerClientRoundTrip(t *testing.T) {
	capabilities := &fakeCapabilities{}
	client, cancel := startGeneralControlAPITestServer(t, capabilities)
	defer cancel()
	ctx := context.Background()

	bases, err := client.ListBases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bases) != 1 || bases[0].Name != "ubuntu" || bases[0].Revision != "rev-1" {
		t.Fatalf("bases = %#v", bases)
	}
	base, err := client.InspectBase(ctx, "ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	if base.Revision != "rev-1" {
		t.Fatalf("base = %#v", base)
	}

	runResult, err := client.Run(ctx, runapp.Spec{WorkspacePath: "/work", Argv: []string{"printf", "ok"}})
	if err != nil {
		t.Fatal(err)
	}
	if runResult.Environment != "run-ok" || runResult.Execution.Stdout != "ok\n" || !runResult.CleanedUp {
		t.Fatalf("run result = %#v", runResult)
	}

	partial, err := client.Run(ctx, runapp.Spec{WorkspacePath: "/work", Argv: []string{"fail"}})
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "recovery_required" {
		t.Fatalf("run error = %v, want recovery_required", err)
	}
	if partial.Execution.ExitCode != 9 || partial.Execution.Stderr != "failed\n" {
		t.Fatalf("partial run result lost across controller boundary: %#v", partial)
	}

	var streamed []eventsapp.Event
	nextOffset, err := client.StreamEvents(ctx, 0, func(event eventsapp.Event) error {
		streamed = append(streamed, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if nextOffset != 20 || len(streamed) != 2 || streamed[1].Type != "completed" {
		t.Fatalf("streamed=%#v next=%d", streamed, nextOffset)
	}

	streamed = nil
	nextOffset, err = client.StreamEvents(ctx, 10, func(event eventsapp.Event) error {
		streamed = append(streamed, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if nextOffset != 20 || len(streamed) != 1 || streamed[0].NextOffset != 20 {
		t.Fatalf("resumed streamed=%#v next=%d", streamed, nextOffset)
	}

	_, err = client.StreamEvents(ctx, 99, func(eventsapp.Event) error { return nil })
	status = nil
	if !errors.As(err, &status) || status.Code != "invalid_argument" {
		t.Fatalf("stream error = %v, want invalid_argument", err)
	}

	request := core.CapabilityRequest{
		Capability:  "demo",
		Action:      "read",
		Resource:    "resource://one",
		Environment: "env-one",
		Attributes:  map[string]string{"authority": "narrow"},
		Parameters:  map[string]string{"format": "json"},
	}
	capabilityResult, err := client.RequestCapability(ctx, request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if capabilityResult.Output != "accepted" || capabilityResult.ExecutionState != core.CapabilitySucceeded {
		t.Fatalf("capability result = %#v", capabilityResult)
	}
	if !reflect.DeepEqual(capabilities.request, request) {
		t.Fatalf("capability request changed across wire:\n got %#v\nwant %#v", capabilities.request, request)
	}
}

func TestGeneralControllerCapabilityApprovalRoundTrip(t *testing.T) {
	capabilities := &fakeCapabilities{}
	client, cancel := startGeneralControlAPITestServer(t, capabilities)
	defer cancel()

	request := core.CapabilityRequest{
		Capability: "demo",
		Action: "approve",
		Resource: "sensitive",
		Attributes: map[string]string{"scope": "one"},
		Parameters: map[string]string{"secret-like-input": "must-not-be-in-prompt"},
	}
	callbackCalls := 0
	result, err := client.RequestCapability(context.Background(), request, func(_ context.Context, approval core.ApprovalRequest) (bool, error) {
		callbackCalls++
		if approval.Reason != "human approval required" || approval.CapabilityRequest.Resource != "sensitive" {
			t.Fatalf("approval = %#v", approval)
		}
		if approval.CapabilityRequest.Parameters != nil {
			t.Fatalf("non-authority parameters leaked into approval prompt: %#v", approval.CapabilityRequest.Parameters)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if callbackCalls != 1 || !capabilities.approved || result.ExecutionState != core.CapabilitySucceeded {
		t.Fatalf("callbackCalls=%d approved=%t result=%#v", callbackCalls, capabilities.approved, result)
	}
	if !reflect.DeepEqual(capabilities.request, request) {
		t.Fatalf("execution request changed across wire:\n got %#v\nwant %#v", capabilities.request, request)
	}
}

func TestGeneralControllerCapabilityApprovalFailsClosedWithoutCallback(t *testing.T) {
	capabilities := &fakeCapabilities{}
	client, cancel := startGeneralControlAPITestServer(t, capabilities)
	defer cancel()

	result, err := client.RequestCapability(context.Background(), core.CapabilityRequest{Capability: "demo", Action: "approve"}, nil)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "denied" {
		t.Fatalf("error = %v, want denied", err)
	}
	if capabilities.approved || result.ExecutionState != core.CapabilityNotExecuted {
		t.Fatalf("approval unexpectedly succeeded: approved=%t result=%#v", capabilities.approved, result)
	}
}

func TestGeneralControllerPreservesCapabilityFailureResult(t *testing.T) {
	capabilities := &fakeCapabilities{}
	client, cancel := startGeneralControlAPITestServer(t, capabilities)
	defer cancel()

	result, err := client.RequestCapability(context.Background(), core.CapabilityRequest{Capability: "demo", Action: "deny"}, nil)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "denied" {
		t.Fatalf("error = %v, want denied", err)
	}
	if result.Output != "accepted" || result.ExecutionState != core.CapabilityNotExecuted {
		t.Fatalf("failure result lost across controller boundary: %#v", result)
	}
}

func TestRegisterGeneralRejectsIncompleteBoundary(t *testing.T) {
	server := control.NewServer()
	if !errors.Is(RegisterGeneral(server, nil, fakeRunner{}, fakeEvents{}, &fakeCapabilities{}), control.ErrInvalidArgument) {
		t.Fatal("nil base service was accepted")
	}
}

func startGeneralControlAPITestServer(t *testing.T, capabilities *fakeCapabilities) (*Client, context.CancelFunc) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := RegisterGeneral(server, fakeBases{}, fakeRunner{}, fakeEvents{}, capabilities); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not stop")
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return client, cancel
}
