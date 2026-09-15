//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspaceinput"
	"github.com/SLktEx/Hacocoon/modules/standard/workflow"
)

type workspaceImportClient interface {
	ImportWorkspace(context.Context, io.Reader, controlapi.WorkspaceImportRequest) (controlapi.WorkspaceImportResult, error)
}

func importWorkspacePath(ctx context.Context, c workspaceImportClient, source, path, name, repo, base, oci string) (workflow.PathReference, error) {
	h, err := workflow.LockReference(ctx, path)
	if err != nil {
		return workflow.PathReference{}, err
	}
	defer h.Close()
	if _, err = h.Load(); !errors.Is(err, os.ErrNotExist) {
		return workflow.PathReference{}, core.ErrAlreadyExists
	}
	if name == "" {
		name = workflow.NewName()
	}
	if oci == "" {
		oci = "auto"
	}
	req := controlapi.WorkspaceImportRequest{Name: name, Repository: repo}
	if err = req.Validate(); err != nil {
		return workflow.PathReference{}, err
	}
	archive, err := workspaceinput.Capture(ctx, source, repo)
	if err != nil {
		return workflow.PathReference{}, err
	}
	defer func() { _ = archive.Close() }()
	ref := workflow.PathReference{Version: 1, Reference: workflow.Reference{Name: name}, Repositories: []string{repo}, State: "importing", Base: core.BaseName(base), OCI: oci}
	if err = h.Save(ref); err != nil {
		return ref, err
	}
	result, err := c.ImportWorkspace(ctx, archive, req)
	ref.State = "recovery-required"
	if result.Workspace != "" {
		ref.Reference = result.Reference
	}
	if err == nil {
		ref.State = "ready"
	}
	return ref, errors.Join(err, h.Save(ref))
}

func workspaceImportCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco workspace import", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	path := flags.String("path", "", cliMessage("detail.path"))
	name := flags.String("name", "", cliMessage("detail.work_name"))
	repo := flags.String("repo", "", cliMessage("detail.repo"))
	base := flags.String("base", "", cliMessage("detail.base"))
	oci := flags.String("oci", "auto", cliMessage("detail.oci"))
	jsonOutput := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *path == "" || *repo == "" {
		commandHelp(diagnostic, "workspace import", cliLanguage())
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	ref, err := importWorkspacePath(ctx, c, flags.Arg(0), *path, *name, *repo, *base, *oci)
	if writeErr := writeCLIResult(out, ref, *jsonOutput); writeErr != nil {
		return 1
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	_, _ = fmt.Fprintln(diagnostic, cliMessage("workflow.import_ready"))
	return 0
}
