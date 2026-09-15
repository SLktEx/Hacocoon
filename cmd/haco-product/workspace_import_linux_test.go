//go:build linux

package main

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspaceinput"
	"github.com/SLktEx/Hacocoon/modules/standard/workflow"
)

type workspaceInputClientFixture struct {
	t       *testing.T
	calls   int
	unknown bool
}

func (c *workspaceInputClientFixture) ImportWorkspace(ctx context.Context, r io.Reader, q controlapi.WorkspaceImportRequest) (controlapi.WorkspaceImportResult, error) {
	c.calls++
	err := workspaceinput.CopyTree(ctx, r, func(_ *tar.Header, r io.Reader) error { _, err := io.Copy(io.Discard, r); return err })
	if err != nil {
		c.t.Fatal(err)
	}
	result := controlapi.WorkspaceImportResult{Reference: workflow.Reference{Name: q.Name, Workspace: "workspace:managed:0123456789abcdef0123456789abcdef"}, Repository: q.Repository, State: "ready"}
	if c.unknown {
		result.State = "recovery-required"
		return result, errors.New("receipt lost")
	}
	return result, nil
}

func TestWorkspaceImportPathRetainsSourceAndNeverReplaysUnconfirmedImport(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "unknown"}[unknown], func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			for _, dir := range []string{".git/objects", ".git/refs"} {
				if err := os.MkdirAll(filepath.Join(source, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for path, body := range map[string]string{".git/config": "[core]\nrepositoryformatversion = 0\n", ".git/HEAD": "ref: refs/heads/main\n", "work": "keep"} {
				if err := os.WriteFile(filepath.Join(source, path), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			client := &workspaceInputClientFixture{t: t, unknown: unknown}
			ref, err := importWorkspacePath(context.Background(), client, source, target, "task", "sample", "", "none")
			if (err != nil) != unknown || ref.Workspace == "" || ref.OCI != "none" {
				t.Fatal(ref, err)
			}
			h, err := workflow.LockReference(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := h.Load()
			h.Close()
			if err != nil || saved.Reference != ref.Reference || (saved.State == "recovery-required") != unknown {
				t.Fatal(saved, err)
			}
			if _, err := importWorkspacePath(context.Background(), client, source, target, "replacement", "sample", "", "auto"); !errors.Is(err, core.ErrAlreadyExists) || client.calls != 1 {
				t.Fatal("replayed import", err, client.calls)
			}
			data, err := os.ReadFile(filepath.Join(source, "work"))
			if err != nil || string(data) != "keep" {
				t.Fatal("source changed", err)
			}
			if unknown {
				openClient := &pathClientFixture{}
				if _, err := openWorkspacePath(context.Background(), openClient, pathOpenOptions{Path: target}); err == nil || openClient.creates != 0 || openClient.opens != 0 {
					t.Fatal("unconfirmed import opened or recreated", err)
				}
			}
		})
	}
}
