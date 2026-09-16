//go:build linux

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

func TestOpenCommandPreparesDirectoryAndReportsCanonicalResult(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed-open"}[fail], func(t *testing.T) {
			path := t.TempDir()
			ref := workflow.Reference{Name: "task", Workspace: "workspace:managed:owned"}
			result := workflow.OpenResult{Reference: ref, Created: true, Environment: core.Environment{Name: "dev", RuntimeRef: "owned-runtime", Workspace: core.Workspace{ID: ref.Workspace, Path: "managed:task"}}}
			requests := workflowCommandServer(t, func(req controlapi.WorkflowRequest) (any, error) {
				switch req.Operation {
				case "prepare":
					if req.Prepare == nil || req.Prepare.Name != "task" || !reflect.DeepEqual(req.Prepare.Repositories, []string{"one", "two"}) {
						t.Error("directory selection changed", req)
					}
					return controlapi.WorkflowResponse{Reference: &ref}, nil
				case "open":
					if req.Open == nil || req.Open.Reference != ref || req.Open.Base != "sample" || req.Open.OCI != "none" {
						t.Error("open changed prepared ownership or options", req)
					}
					if fail {
						return nil, control.NewStatusError("recovery_required", "retained Environment")
					}
					return controlapi.WorkflowResponse{Open: &result}, nil
				default:
					t.Error("unexpected operation", req)
					return nil, control.ErrInvalidArgument
				}
			})
			code, out, diagnostic := captureRun(t, "open", "--client", "none", "--json", "--repo", "one,two", "--name", "task", "--base", "sample", "--oci", "none", path)
			if len(requests) != 2 {
				t.Fatal("open skipped or repeated a lifecycle operation", len(requests))
			}
			saved := loadWorkflowReference(t, path)
			if saved.Reference != ref || saved.State != "ready" {
				t.Fatal("prepared Workspace reference lost", saved)
			}
			if fail {
				if code != 1 || out != "" || diagnostic == "" {
					t.Fatal("uncertain open reported success", code, out, diagnostic)
				}
				return
			}
			var got workflow.OpenResult
			if code != 0 || diagnostic != "" || json.Unmarshal([]byte(out), &got) != nil || !reflect.DeepEqual(got, result) || saved.Base != "sample" || saved.OCI != "none" {
				t.Fatal("machine result or saved options changed", code, out, diagnostic, saved)
			}
		})
	}
}

func TestOpenCommandRejectsInvalidModeBeforePreparation(t *testing.T) {
	path := t.TempDir()
	requests := workflowCommandServer(t, func(controlapi.WorkflowRequest) (any, error) {
		return nil, control.ErrInvalidArgument
	})
	for _, args := range [][]string{
		{"--unknown"}, {"--client", "other", "dev"}, {"dev", "extra"},
		{"--json", "dev"}, {"--client", "none", "dev"}, {"--repo", "one", "dev"},
		{"--name", "task", "dev"}, {"--base", "base", "dev"}, {"--oci", "none", "dev"},
		{"--close", "dev"}, {"--no-browser", "dev"}, {"--port", "0", "dev"}, {"--port", "65536", "dev"}, {"--port", "8080", "--client", "ssh", "dev"},
		{"--json", path}, {"--close", path}, {"--no-browser", path},
		{"--port", "0", path}, {"--port", "65536", path}, {"--port", "8080", "--client", "ssh", path},
	} {
		code, _, _ := captureRun(t, append([]string{"open"}, args...)...)
		if code != 2 || len(requests) != 0 {
			t.Fatal("invalid invocation reached preparation", args, code, len(requests))
		}
		if _, err := os.Stat(filepath.Join(path, workflow.ReferenceFile)); !os.IsNotExist(err) {
			t.Fatal("invalid invocation wrote a Workspace reference", args, err)
		}
	}
}
