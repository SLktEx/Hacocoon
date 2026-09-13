//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/workflow"
)

type workflowClient interface {
	WorkspaceWorkflow(context.Context, controlapi.WorkflowRequest) (controlapi.WorkflowResponse, error)
}
type pathOpenOptions struct{ Path, Name, Repositories, Base, OCI string }

func workspacePath(value string) bool {
	if value == "." || value == ".." || filepath.IsAbs(value) || strings.ContainsAny(value, "/\\") {
		return true
	}
	info, err := os.Stat(value)
	return err == nil && info.IsDir()
}
func preparePath(ctx context.Context, c workflowClient, h *workflow.ReferenceHandle, opts pathOpenOptions) (workflow.PathReference, error) {
	ref, err := h.Load()
	if errors.Is(err, os.ErrNotExist) {
		if opts.Repositories == "" {
			return ref, fmt.Errorf("directory has no Workspace reference; select Host repositories with --repo first,second; existing directory files are not imported")
		}
		name := opts.Name
		if name == "" {
			name = workflow.NewName()
		}
		oci := opts.OCI
		if oci == "" {
			oci = "auto"
		}
		ref = workflow.PathReference{Version: 1, Reference: workflow.Reference{Name: name}, State: "preparing", Repositories: strings.Split(opts.Repositories, ","), Base: core.BaseName(opts.Base), OCI: oci}
		if err = h.Save(ref); err != nil {
			return ref, err
		}
	} else if err != nil {
		return ref, err
	}
	if opts.Name != "" && opts.Name != ref.Name {
		return ref, core.ErrCapabilityStale
	}
	if opts.Repositories != "" && strings.Join(ref.Repositories, ",") != opts.Repositories {
		return ref, fmt.Errorf("existing reference has different repository selection: %w", core.ErrAlreadyExists)
	}
	if ref.State == "preparing" {
		response, e := c.WorkspaceWorkflow(ctx, controlapi.WorkflowRequest{Operation: "prepare", Prepare: &workflow.PrepareSpec{Name: ref.Name, Repositories: ref.Repositories}})
		if response.Reference != nil && response.Reference.Workspace != "" {
			ref.Reference = *response.Reference
		}
		if e != nil {
			// Keep the preparation name stable across lost responses. Re-entry uses
			// idempotent preparation, which refuses incomplete provider ownership.
			return ref, errors.Join(e, h.Save(ref))
		}
		if response.Reference == nil || response.Reference.Workspace == "" {
			return ref, core.ErrIncompatibleState
		}
		ref.State = "ready"
		if err = h.Save(ref); err != nil {
			return ref, err
		}
	}
	if ref.State != "ready" {
		return ref, fmt.Errorf("reference %s requires recovery; inspect workspace list and snapshot list: %w", ref.Name, core.ErrRecoveryRequired)
	}
	return ref, nil
}
func openWorkspacePath(ctx context.Context, c workflowClient, opts pathOpenOptions) (workflow.OpenResult, error) {
	var result workflow.OpenResult
	h, err := workflow.LockReference(ctx, opts.Path)
	if err != nil {
		return result, err
	}
	defer h.Close()
	ref, err := preparePath(ctx, c, h, opts)
	if err != nil {
		return result, err
	}
	base, oci := ref.Base, ref.OCI
	expected := ref.Resource
	if opts.Base != "" {
		base = core.BaseName(opts.Base)
	}
	if opts.OCI != "" {
		oci = opts.OCI
		expected = core.PersistentResourceRef{}
	}
	response, err := c.WorkspaceWorkflow(ctx, controlapi.WorkflowRequest{Operation: "open", Open: &workflow.OpenSpec{Reference: ref.Reference, Base: base, OCI: oci, ExpectedResource: expected}})
	if response.Open != nil {
		result = *response.Open
	}
	if err != nil {
		return result, err
	}
	if result.Workspace != ref.Workspace || result.Environment.Workspace.ID != ref.Workspace {
		return result, core.ErrCapabilityStale
	}
	// Persist preferences only after successful canonical open. On a lost response
	// a repeat adopts the same exact owned Env.
	ref.Base, ref.OCI = base, oci
	ref.Resource = result.Environment.PersistentResource
	return result, h.Save(ref)
}
func workflowCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco workspace "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	path := flags.String("path", "", cliMessage("detail.path"))
	name := flags.String("name", "", cliMessage("detail.work_name"))
	repos := flags.String("repo", "", cliMessage("detail.repos"))
	base := flags.String("base", "", cliMessage("detail.base"))
	oci := flags.String("oci", "", cliMessage("detail.oci"))
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *path == "" || (args[0] == "prepare" && flags.NArg() != 0) || (args[0] == "fork" && (flags.NArg() != 1 || *repos != "" || *oci != "")) {
		commandHelp(diagnostic, "workspace "+args[0], cliLanguage())
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if args[0] == "prepare" {
		h, e := workflow.LockReference(ctx, *path)
		if e != nil {
			fmt.Fprintln(diagnostic, "haco:", e)
			return 1
		}
		defer h.Close()
		ref, e := preparePath(ctx, c, h, pathOpenOptions{Path: *path, Name: *name, Repositories: *repos, Base: *base, OCI: *oci})
		_ = json.NewEncoder(out).Encode(ref)
		if e != nil {
			fmt.Fprintln(diagnostic, "haco:", e)
			return 1
		}
		return 0
	}
	var source workflow.Reference
	if workspacePath(flags.Arg(0)) {
		h, e := workflow.LockReference(ctx, flags.Arg(0))
		if e != nil {
			fmt.Fprintln(diagnostic, "haco:", e)
			return 1
		}
		ref, e := h.Load()
		h.Close()
		if e != nil || ref.State != "ready" {
			fmt.Fprintln(diagnostic, "haco: source reference is not ready", e)
			return 1
		}
		source = ref.Reference
	} else {
		response, e := c.WorkspaceWorkflow(ctx, controlapi.WorkflowRequest{Operation: "reference", Reference: &workflow.Reference{Name: flags.Arg(0)}})
		if e != nil || response.Reference == nil {
			fmt.Fprintln(diagnostic, "haco: source reference unavailable", e)
			return 1
		}
		source = *response.Reference
	}
	h, err := workflow.LockReference(ctx, *path)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	defer h.Close()
	if _, e := h.Load(); !errors.Is(e, os.ErrNotExist) {
		fmt.Fprintln(diagnostic, "haco: destination reference already exists or is unsafe; inspect it before retrying")
		return 1
	}
	if *name == "" {
		*name = workflow.NewName()
	}
	ref := workflow.PathReference{Version: 1, Reference: workflow.Reference{Name: *name}, State: "forking", OCI: "none"}
	if err = h.Save(ref); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	response, err := c.WorkspaceWorkflow(ctx, controlapi.WorkflowRequest{Operation: "fork", Reference: &source, Target: *name})
	if response.Fork != nil {
		fork := response.Fork
		if fork.Workspace != "" {
			ref.Reference = fork.Reference
		}
		ref.State, ref.OCI, ref.Base, ref.TemporarySnapshot = fork.State, fork.OCI, fork.Base, fork.TemporarySnapshot
		ref.Resource = fork.Resource
		if ref.State == "" {
			ref.State = "recovery-required"
		}
		if ref.OCI == "" {
			ref.OCI = "none"
		}
		if *base != "" {
			ref.Base = core.BaseName(*base)
		}
	} else {
		ref.State = "recovery-required"
	}
	err = errors.Join(err, h.Save(ref))
	_ = json.NewEncoder(out).Encode(ref)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	fmt.Fprintln(diagnostic, "Workspace fork ready; source stays stopped. Open the destination directory to create its Env.")
	return 0
}
