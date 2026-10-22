//go:build linux

package cli

import (
	"context"
	"errors"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
	"testing"
)

func TestOpenRejectsReplacedPreparationReceipt(t *testing.T) {
	path := t.TempDir()
	original := workflow.PathReference{Version: 1, State: "preparing", Reference: workflow.Reference{Name: "work", Workspace: "workspace:managed:original"}, Repositories: []string{"one"}, OCI: "auto"}
	saveWorkflowReference(t, path, original)
	for _, changed := range []workflow.Reference{
		{Name: "different", Workspace: original.Workspace},
		{Name: original.Name, Workspace: "workspace:managed:replacement"},
	} {
		c := workflowResponseFixture(func(req controlapi.WorkflowRequest) controlapi.WorkflowResponse {
			if req.Operation != "prepare" {
				t.Fatal("changed preparation reached open")
			}
			return controlapi.WorkflowResponse{Reference: &changed}
		})
		if _, err := openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path}); !errors.Is(err, core.ErrCapabilityStale) {
			t.Fatal("changed preparation adopted", err)
		}
		if saved := loadWorkflowReference(t, path); saved.Reference != original.Reference || saved.State != "preparing" {
			t.Fatal("changed receipt replaced durable identity", saved)
		}
	}
}

type workflowResponseFixture func(controlapi.WorkflowRequest) controlapi.WorkflowResponse

func (f workflowResponseFixture) WorkspaceWorkflow(_ context.Context, req controlapi.WorkflowRequest) (controlapi.WorkflowResponse, error) {
	return f(req), nil
}
