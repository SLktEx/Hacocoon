package controlapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

type workflowAPIFixture struct {
	calls     int
	selection []string
}

func (f *workflowAPIFixture) Reference(context.Context, string) (workflow.Reference, error) {
	f.calls++
	return workflow.Reference{}, nil
}
func (f *workflowAPIFixture) Prepare(context.Context, workflow.PrepareSpec) (workflow.Reference, error) {
	f.calls++
	return workflow.Reference{}, nil
}
func (f *workflowAPIFixture) Open(context.Context, workflow.OpenSpec) (workflow.OpenResult, error) {
	f.calls++
	return workflow.OpenResult{}, nil
}
func (f *workflowAPIFixture) Fork(_ context.Context, _ workflow.Reference, _ string, selection []string) (workflow.ForkResult, error) {
	f.calls++
	f.selection = selection
	return workflow.ForkResult{Reference: workflow.Reference{Name: "fork", Workspace: "owned"}, State: "recovery-required", TemporarySnapshot: "retained"}, core.ErrRecoveryRequired
}
func TestWorkflowTransportRetainsRecoveryAndRejectsAmbiguousRequests(t *testing.T) {
	f := &workflowAPIFixture{}
	server := control.NewServer()
	if err := RegisterWorkflow(server, f); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "workflow.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	client, err := NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.WorkspaceWorkflow(ctx, WorkflowRequest{Operation: "fork", Reference: &workflow.Reference{Name: "source", Workspace: "owned-source"}, Target: "fork", Repositories: []string{"one", "three"}})
	if err == nil || response.Fork == nil || response.Fork.Workspace != "owned" || response.Fork.TemporarySnapshot != "retained" {
		t.Fatal(response, err)
	}
	for _, raw := range []string{
		`{"operation":"reference","reference":{"name":"source"},"repositories":["injected"]}`,
		`null`, `{}`, `{"operation":"unknown"}`,
		`{"operation":"fork","reference":{"name":"source"},"target":"fork"}`,
		`{"operation":"prepare","prepare":{"name":"work","repositories":["repo"],"native_ref":"injected"}}`,
		`{"operation":"open","open":{"name":"work","workspace":"owned"},"prepare":{"name":"other"}}`,
		`{"operation":"reference","reference":{"name":"source","workspace":"unexpected"}}`,
		`{"operation":"fork","reference":{"name":"source","workspace":"owned"},"target":"fork","owner":"injected"}`,
	} {
		var result any
		if err := client.wire.Call(ctx, MethodWorkflow, json.RawMessage(raw), &result); err == nil {
			t.Fatal("unsafe request accepted", raw)
		}
	}
	if len(f.selection) != 2 || f.selection[1] != "three" {
		t.Fatal("lost selection", f.selection)
	}
	if f.calls != 1 {
		t.Fatal("invalid input reached mutation", f.calls)
	}
}
