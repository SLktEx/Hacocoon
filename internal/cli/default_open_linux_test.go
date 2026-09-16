//go:build linux

package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

type defaultOpenFixture struct {
	pathClientFixture
	sources []gitrepo.SourceUse
	listErr error
}

func (c *defaultOpenFixture) RepositoryManage(context.Context, controlapi.RepositoryManageRequest) (controlapi.RepositoryManageResponse, error) {
	return controlapi.RepositoryManageResponse{Sources: c.sources}, c.listErr
}

func readySources(names ...string) []gitrepo.SourceUse {
	var sources []gitrepo.SourceUse
	for _, name := range names {
		sources = append(sources, gitrepo.SourceUse{Source: gitrepo.Object{Kind: "repo", ID: name, State: "ready"}})
	}
	return sources
}

func TestDefaultOpenReusesStableSortedSelectionAndDurableReceipts(t *testing.T) {
	setCLITestLocale(t, "C")
	c := &defaultOpenFixture{sources: readySources("web", "api")}
	path := t.TempDir()
	c.lostPrepare = true
	var output bytes.Buffer
	if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", &output); err == nil {
		t.Fatal("lost preparation was hidden")
	}
	pending := loadWorkflowReference(t, path)
	if pending.State != "preparing" || !reflect.DeepEqual(pending.Repositories, []string{"api", "web"}) {
		t.Fatal("selection not durable before mutation", pending)
	}
	for i := 0; i < 2; i++ {
		c.sources = readySources("api", "web")
		result, err := openDefaultWorkspace(context.Background(), c, path, "", "", &output)
		if err != nil || result.Name != pending.Name || result.Workspace != c.source.Workspace {
			t.Fatal("retry lost identity", result, err)
		}
	}
	if c.creates != 1 || c.last.Base != "" || c.last.OCI != "auto" {
		t.Fatal("default provider/resource path changed", c)
	}
	if !strings.Contains(output.String(), "Preparing project files") || !strings.Contains(output.String(), "development environment") {
		t.Fatal("missing high-level progress", output.String())
	}
	// Adding a source must never silently replace the user's existing edited
	// collection. Immutable membership remains an explicit fork operation.
	c.sources = readySources("api", "web", "third")
	opens := c.opens
	if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); !errors.Is(err, core.ErrAlreadyExists) || c.opens != opens {
		t.Fatal("changed membership silently opened/replaced work", err)
	}
}

func TestDefaultOpenRefusesInvalidInventoryBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sources []gitrepo.SourceUse
		err     error
	}{
		{"empty", nil, nil},
		{"unavailable", nil, core.ErrRuntimeUnavailable},
		{"duplicate", readySources("one", "one"), nil},
		{"option", readySources("--evil"), nil},
		{"traversal", readySources("../evil"), nil},
		{"limit", readySources("a", "b", "c", "d", "e", "f", "g", "h", "i"), nil},
		{"incomplete", []gitrepo.SourceUse{{Source: gitrepo.Object{Kind: "repo", ID: "one", State: "creating"}}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &defaultOpenFixture{sources: tc.sources, listErr: tc.err}
			path := t.TempDir()
			if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); err == nil || c.prepare != 0 || c.opens != 0 {
				t.Fatal("invalid inventory reached mutation", err)
			}
			if _, err := os.Stat(filepath.Join(path, workflow.ReferenceFile)); !os.IsNotExist(err) {
				t.Fatal("invalid inventory saved a session", err)
			}
		})
	}
}

func TestDefaultOpenConcurrentCallsAndCancellation(t *testing.T) {
	c := &defaultOpenFixture{sources: readySources("one", "two")}
	path := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c.creates != 1 || c.opens != 8 {
		t.Fatal("concurrent session created duplicates", c.creates, c.opens)
	}
	h, err := workflow.LockReference(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := openDefaultWorkspace(ctx, c, path, "", "", io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting lock ignored cancellation", err)
	}
}

func TestDefaultOpenRefusesUnsafeReferencesAndKeepsFailedWork(t *testing.T) {
	for _, attack := range []string{"directory-symlink", "reference-symlink", "reference-hardlink", "writable-directory"} {
		t.Run(attack, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".haco-default")
			target := t.TempDir()
			if attack == "directory-symlink" {
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(target, "keep")
				if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
				switch attack {
				case "reference-symlink":
					if err := os.Symlink(file, filepath.Join(path, workflow.ReferenceFile)); err != nil {
						t.Fatal(err)
					}
				case "reference-hardlink":
					if err := os.Link(file, filepath.Join(path, workflow.ReferenceFile)); err != nil {
						t.Fatal(err)
					}
				case "writable-directory":
					if err := os.Chmod(path, 0777); err != nil {
						t.Fatal(err)
					}
				}
			}
			c := &defaultOpenFixture{sources: readySources("one")}
			if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); err == nil || c.creates != 0 {
				t.Fatal("unsafe reference accepted", err)
			}
		})
	}
	c := &defaultOpenFixture{sources: readySources("one", "two")}
	c.fail = core.ErrPolicyDenied
	path := t.TempDir()
	if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal("denial changed", err)
	}
	saved := loadWorkflowReference(t, path)
	if saved.State != "ready" || saved.Workspace == "" {
		t.Fatal("denial discarded work", saved)
	}
	c.fail = nil
	if _, err := openDefaultWorkspace(context.Background(), c, path, "", "", io.Discard); err != nil || c.creates != 1 {
		t.Fatal("authorized retry repeated copy", err)
	}
}

type workflowResponseFixture func(controlapi.WorkflowRequest) controlapi.WorkflowResponse

func (f workflowResponseFixture) WorkspaceWorkflow(_ context.Context, req controlapi.WorkflowRequest) (controlapi.WorkflowResponse, error) {
	return f(req), nil
}

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
