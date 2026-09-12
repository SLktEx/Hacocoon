//go:build linux

package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/workflow"
	"os"
	"path/filepath"
	"testing"
)

type pathClientFixture struct {
	prepare, creates, opens int
	lostPrepare, lostOpen   bool
	source                  workflow.Reference
	exists                  bool
	last                    workflow.OpenSpec
	fail                    error
}

func (c *pathClientFixture) WorkspaceWorkflow(_ context.Context, req controlapi.WorkflowRequest) (controlapi.WorkflowResponse, error) {
	switch req.Operation {
	case "prepare":
		c.prepare++
		if !c.exists {
			c.creates++
			c.exists = true
			c.source = workflow.Reference{Name: req.Prepare.Name, Workspace: "workspace:managed:owner"}
		}
		if c.lostPrepare {
			c.lostPrepare = false
			return controlapi.WorkflowResponse{}, errors.New("lost response")
		}
		return controlapi.WorkflowResponse{Reference: &c.source}, nil
	case "open":
		c.opens++
		c.last = *req.Open
		if c.fail != nil {
			return controlapi.WorkflowResponse{}, c.fail
		}
		if c.lostOpen {
			c.lostOpen = false
			return controlapi.WorkflowResponse{}, errors.New("lost response")
		}
		return controlapi.WorkflowResponse{Open: &workflow.OpenResult{Reference: c.source, Environment: core.Environment{Name: "same-env", Workspace: core.Workspace{ID: c.source.Workspace}}}}, nil
	}
	panic("unexpected operation")
}
func TestPathOpenResumesLostPreparationAndKeepsNoOCIChoice(t *testing.T) {
	path := t.TempDir()
	c := &pathClientFixture{lostPrepare: true}
	opts := pathOpenOptions{Path: path, Repositories: "one,two", Name: "task", OCI: "none", Base: "haco/ubuntu-26.04"}
	if _, err := openWorkspacePath(context.Background(), c, opts); err == nil {
		t.Fatal("lost response hidden")
	}
	result, err := openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path})
	if err != nil || result.Workspace != c.source.Workspace || c.creates != 1 || c.prepare != 2 || c.last.OCI != "none" {
		t.Fatal(result, err, c)
	}
	c.lostOpen = true
	if _, err = openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path}); err == nil {
		t.Fatal("lost open hidden")
	}
	if _, err = openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path}); err != nil || c.creates != 1 {
		t.Fatal(err, c)
	}
}
func TestPathDoesNotImplicitlyImportOrReplaceOriginalData(t *testing.T) {
	path := t.TempDir()
	file := filepath.Join(path, "source")
	os.WriteFile(file, []byte("keep"), 0644)
	c := &pathClientFixture{}
	if _, err := openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path}); err == nil || c.creates != 0 {
		t.Fatal("implicit import", err)
	}
	raw, _ := os.ReadFile(file)
	if string(raw) != "keep" {
		t.Fatal("source moved")
	}
	if _, err := openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path, Repositories: "one", Name: "task", OCI: "none"}); err != nil {
		t.Fatal(err)
	}
	c.fail = core.ErrIncompatibleState
	if _, err := openWorkspacePath(context.Background(), c, pathOpenOptions{Path: path, Base: "haco/ubuntu-24.04"}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	h, err := workflow.LockReference(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ref, err := h.Load()
	if err != nil || ref.Base != "" || ref.OCI != "none" {
		t.Fatal("failed open changed preferences", ref, err)
	}
}
