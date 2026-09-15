//go:build linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

// Exercise the CLI over its ordinary Unix transport. The controller fixture
// supplies operation outcomes; the real path reference owns local persistence.
func workflowCommandServer(t *testing.T, respond func(controlapi.WorkflowRequest) (any, error)) <-chan controlapi.WorkflowRequest {
	t.Helper()
	requests := make(chan controlapi.WorkflowRequest, 32)
	server := control.NewServer()
	if err := server.Register(controlapi.MethodWorkflow, func(_ context.Context, raw json.RawMessage) (any, error) {
		var req controlapi.WorkflowRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, err
		}
		requests <- req
		return respond(req)
	}); err != nil {
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
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	setCLITestLocale(t, "C")
	return requests
}

func loadWorkflowReference(t *testing.T, path string) workflow.PathReference {
	t.Helper()
	h, err := workflow.LockReference(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ref, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func saveWorkflowReference(t *testing.T, path string, ref workflow.PathReference) {
	t.Helper()
	h, err := workflow.LockReference(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Save(ref); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspacePrepareCommandPersistsBeforeDispatchAndReopensWithoutMutation(t *testing.T) {
	path := t.TempDir()
	data := filepath.Join(path, "existing-data")
	if err := os.WriteFile(data, []byte("keep local data"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := workflowCommandServer(t, func(req controlapi.WorkflowRequest) (any, error) {
		raw, err := os.ReadFile(filepath.Join(path, workflow.ReferenceFile))
		var saved workflow.PathReference
		if err != nil || json.Unmarshal(raw, &saved) != nil || saved.State != "preparing" || req.Prepare == nil || saved.Name != req.Prepare.Name || !reflect.DeepEqual(saved.Repositories, req.Prepare.Repositories) {
			return nil, errors.New("preparation was dispatched without its durable reference")
		}
		return controlapi.WorkflowResponse{Reference: &workflow.Reference{Name: saved.Name, Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32))}}, nil
	})
	var out, diagnostic bytes.Buffer
	args := []string{"prepare", "--path", path, "--repo", "one,two", "--base", "haco/ubuntu-26.04", "--json"}
	if code := workflowCommand(context.Background(), args, &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	ref := loadWorkflowReference(t, path)
	var printed workflow.PathReference
	if err := json.Unmarshal(out.Bytes(), &printed); err != nil || !reflect.DeepEqual(ref, printed) || ref.State != "ready" || ref.OCI != "auto" || ref.Base != "haco/ubuntu-26.04" || !strings.HasPrefix(ref.Name, "work-") {
		t.Fatal("preparation result disagrees with durable reference", ref, printed, err)
	}
	if req := <-requests; req.Operation != "prepare" || req.Prepare.Name != ref.Name {
		t.Fatal(req)
	}
	for _, extra := range [][]string{nil, {"--name", "different"}, {"--repo", "two,one"}} {
		out.Reset()
		diagnostic.Reset()
		code := workflowCommand(context.Background(), append([]string{"prepare", "--path", path, "--json"}, extra...), &out, &diagnostic)
		if (code == 0) != (extra == nil) || !reflect.DeepEqual(loadWorkflowReference(t, path), ref) || len(requests) != 0 {
			t.Fatal("re-entry changed existing ownership or preferences", extra, code, diagnostic.String())
		}
	}
	if raw, err := os.ReadFile(data); err != nil || string(raw) != "keep local data" {
		t.Fatal("preparation changed local data", err)
	}
}

func TestWorkspaceForkCommandRetainsReceiptsAndNeverRepeatsAnIncompleteCopy(t *testing.T) {
	for _, mode := range []string{"ready", "partial-error", "missing-result", "missing-state", "recovery-without-error", "transport-error"} {
		t.Run(mode, func(t *testing.T) {
			sourcePath, destination := t.TempDir(), t.TempDir()
			source := workflow.PathReference{Version: 1, Reference: workflow.Reference{Name: "source", Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32))}, State: "ready", OCI: "none"}
			saveWorkflowReference(t, sourcePath, source)
			fork := workflow.ForkResult{Reference: workflow.Reference{Name: "next", Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("b", 32))}, State: "ready", OCI: "oci:fork", Base: "haco/ubuntu-26.04", Repositories: []string{"one", "three"}, Resource: core.PersistentResourceRef{ID: "oci:fork", Owner: strings.Repeat("c", 32)}}
			response := any(controlapi.WorkflowResponse{Fork: &fork})
			if mode == "partial-error" {
				fork.State = "recovery-required"
				fork.TemporarySnapshot = "snap-retained"
				response = map[string]any{"fork": fork, "error": map[string]string{"code": "recovery_required", "message": "copy completion is unknown"}}
			}
			if mode == "missing-result" {
				response = controlapi.WorkflowResponse{}
			}
			if mode == "missing-state" {
				fork.State, fork.OCI = "", ""
			}
			if mode == "recovery-without-error" {
				fork.State = "recovery-required"
			}
			requests := workflowCommandServer(t, func(req controlapi.WorkflowRequest) (any, error) {
				if req.Operation != "fork" || req.Reference == nil || *req.Reference != source.Reference || req.Target != "next" || !reflect.DeepEqual(req.Repositories, fork.Repositories) {
					return nil, errors.New("fork changed source ownership or selected membership")
				}
				raw, err := os.ReadFile(filepath.Join(destination, workflow.ReferenceFile))
				var saved workflow.PathReference
				if err != nil || json.Unmarshal(raw, &saved) != nil || saved.State != "forking" || saved.Name != "next" {
					return nil, errors.New("fork was dispatched before the recovery reference was saved")
				}
				if mode == "transport-error" {
					return nil, control.NewStatusError("unavailable", "operation reply lost")
				}
				return response, nil
			})
			var out, diagnostic bytes.Buffer
			args := []string{"fork", "--path", destination, "--name", "next", "--repo", "one,three", "--base", "haco/ubuntu-24.04", "--json", sourcePath}
			code := workflowCommand(context.Background(), args, &out, &diagnostic)
			if (code == 0) != (mode == "ready") {
				t.Errorf("incomplete copy reported success: code=%d diagnostic=%q", code, diagnostic.String())
			}
			if mode != "ready" && strings.Contains(diagnostic.String(), cliMessage("workflow.fork_ready")) {
				t.Error("incomplete copy printed the ready instruction")
			}
			ref := loadWorkflowReference(t, destination)
			var printed workflow.PathReference
			if err := json.Unmarshal(out.Bytes(), &printed); err != nil || !reflect.DeepEqual(printed, ref) || ref.Name != "next" {
				t.Fatal("recovery receipt was not exposed and saved", printed, ref, err)
			}
			if mode != "missing-result" && mode != "transport-error" && (ref.Workspace != fork.Workspace || ref.Resource != fork.Resource || ref.Base != "haco/ubuntu-24.04" || ref.TemporarySnapshot != fork.TemporarySnapshot) {
				t.Fatal("fork lost exact retained identities or Base override", ref, fork)
			}
			if mode != "ready" && ref.State != "recovery-required" {
				t.Fatal("incomplete result became ready", ref)
			}
			if (mode == "missing-result" || mode == "missing-state") && ref.OCI != "none" {
				t.Fatal("unknown OCI selection became automatic", ref)
			}
			<-requests
			out.Reset()
			diagnostic.Reset()
			if code = workflowCommand(context.Background(), args, &out, &diagnostic); code != 1 || len(requests) != 0 || !reflect.DeepEqual(ref, loadWorkflowReference(t, destination)) {
				t.Fatal("retry dispatched a second copy or replaced the receipt", code, diagnostic.String())
			}
			if got := loadWorkflowReference(t, sourcePath); !reflect.DeepEqual(got, source) {
				t.Fatal("fork altered source reference", got)
			}
		})
	}
}

func TestWorkspaceForkByNamePinsResolvedSourceAndKeepsNoOCIChoice(t *testing.T) {
	destination := t.TempDir()
	source := workflow.Reference{Name: "registered-source", Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32))}
	requests := workflowCommandServer(t, func(req controlapi.WorkflowRequest) (any, error) {
		if req.Operation == "reference" {
			return controlapi.WorkflowResponse{Reference: &source}, nil
		}
		if req.Operation != "fork" || req.Reference == nil || *req.Reference != source || req.Target == "" {
			return nil, errors.New("fork did not use the resolved owner")
		}
		return controlapi.WorkflowResponse{Fork: &workflow.ForkResult{Reference: workflow.Reference{Name: req.Target, Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("b", 32))}, State: "ready", OCI: "none", Base: "haco/ubuntu-26.04", Repositories: []string{"one"}}}, nil
	})
	var out, diagnostic bytes.Buffer
	if code := workflowCommand(context.Background(), []string{"fork", "--path", destination, source.Name}, &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	ref := loadWorkflowReference(t, destination)
	if ref.State != "ready" || ref.OCI != "none" || ref.Base != "haco/ubuntu-26.04" || ref.Name == source.Name || ref.Workspace == source.Workspace || !strings.Contains(out.String(), ref.Name) {
		t.Fatal("fork did not create an independent reference", ref, out.String())
	}
	if first, second := <-requests, <-requests; first.Operation != "reference" || first.Reference.Name != source.Name || second.Operation != "fork" || second.Target != ref.Name || len(requests) != 0 {
		t.Fatal(first, second)
	}
}

func TestWorkspaceCommandsRefuseUnsafeReferencesBeforeCopy(t *testing.T) {
	for _, mode := range []string{"missing-repositories", "incomplete-preparation", "malformed-preparation", "unsafe-directory", "source-not-ready", "source-missing", "source-unavailable", "destination-exists", "destination-malformed", "duplicate-repositories"} {
		t.Run(mode, func(t *testing.T) {
			sourcePath, destination := t.TempDir(), t.TempDir()
			source := workflow.PathReference{Version: 1, Reference: workflow.Reference{Name: "source", Workspace: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32))}, State: "ready", OCI: "none"}
			saveWorkflowReference(t, sourcePath, source)
			args := []string{"fork", "--path", destination, "--name", "next", sourcePath}
			wantCode := 1
			switch mode {
			case "missing-repositories":
				args = []string{"prepare", "--path", destination}
			case "incomplete-preparation":
				incomplete := source
				incomplete.State = "forking"
				saveWorkflowReference(t, destination, incomplete)
				args = []string{"prepare", "--path", destination}
			case "malformed-preparation", "destination-malformed":
				if err := os.WriteFile(filepath.Join(destination, workflow.ReferenceFile), []byte("partial JSON"), 0600); err != nil {
					t.Fatal(err)
				}
				if mode == "malformed-preparation" {
					args = []string{"prepare", "--path", destination, "--repo", "one"}
				}
			case "unsafe-directory":
				if err := os.Chmod(destination, 0777); err != nil {
					t.Fatal(err)
				}
				args = []string{"prepare", "--path", destination, "--repo", "one"}
			case "source-not-ready":
				source.State = "recovery-required"
				saveWorkflowReference(t, sourcePath, source)
			case "source-missing":
				args[len(args)-1] = filepath.Join(sourcePath, "missing")
			case "source-unavailable":
				args[len(args)-1] = "missing-source-name"
			case "destination-exists":
				saveWorkflowReference(t, destination, source)
			case "duplicate-repositories":
				args = []string{"fork", "--path", destination, "--repo", "one,one", sourcePath}
				wantCode = 2
			}
			before, beforeErr := os.ReadFile(filepath.Join(destination, workflow.ReferenceFile))
			requests := workflowCommandServer(t, func(controlapi.WorkflowRequest) (any, error) {
				return nil, control.NewStatusError("not_found", "source unavailable")
			})
			var out, diagnostic bytes.Buffer
			if code := workflowCommand(context.Background(), args, &out, &diagnostic); code != wantCode || diagnostic.Len() == 0 {
				t.Fatal(code, diagnostic.String())
			}
			after, afterErr := os.ReadFile(filepath.Join(destination, workflow.ReferenceFile))
			if !bytes.Equal(before, after) || errors.Is(beforeErr, os.ErrNotExist) != errors.Is(afterErr, os.ErrNotExist) {
				t.Fatal("refusal replaced or created a recovery reference", string(before), string(after), afterErr)
			}
			for len(requests) != 0 {
				if req := <-requests; req.Operation != "reference" {
					t.Fatal("unsafe request reached remote mutation", req)
				}
			}
			if got := loadWorkflowReference(t, sourcePath); !reflect.DeepEqual(got, source) {
				t.Fatal("refusal changed source ownership", got)
			}
		})
	}
}
