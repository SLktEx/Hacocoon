package main

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

type lifecycleClient interface {
	environmentExecutor
	StopEnvironment(context.Context, string) error
	StartEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
	CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error)
}

func checkRetainedWorkspace(ctx context.Context, c lifecycleClient, old core.Environment) error {
	if old.AccessMode != core.WorkspaceReadWrite || old.PersistentResource.ID != "" {
		return fmt.Errorf("lifecycle fixture requires independent external read/write Workspace")
	}
	run := func(phase, script string) error {
		result, err := c.ExecEnvironment(ctx, old.Name, []string{"/bin/sh", "-ec", script})
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated || strings.TrimSpace(result.Stdout) != "verified" {
			return fmt.Errorf("lifecycle %s failed (exit %d, transport_failed=%t)", phase, result.ExitCode, err != nil)
		}
		return nil
	}
	if err := run("write", `test ! -e /workspace/work.txt
test ! -e /root/haco-environment-only
printf 'uncommitted-work' > /workspace/work.txt
printf 'environment-only' > /root/haco-environment-only
printf verified`); err != nil {
		return err
	}
	if err := c.StopEnvironment(ctx, old.Name); err != nil {
		return fmt.Errorf("lifecycle stop failed: %w", err)
	}
	if err := c.StartEnvironment(ctx, old.Name); err != nil {
		return fmt.Errorf("lifecycle start failed: %w", err)
	}
	if err := run("resume", `test "$(cat /workspace/work.txt)" = uncommitted-work || exit 41
test "$(cat /root/haco-environment-only)" = environment-only || exit 42
printf verified`); err != nil {
		return err
	}
	if err := c.StopEnvironment(ctx, old.Name); err != nil {
		return err
	}
	if err := c.DeleteEnvironment(ctx, old.Name); err != nil {
		return err
	}
	request := controlapi.EnvironmentCreateRequest{Name: old.Name, WorkspacePath: old.Workspace.Path, AccessMode: old.AccessMode, Resources: old.Resources}
	if old.Base != nil {
		request.Base = old.Base.Name
	}
	created, err := c.CreateEnvironment(ctx, request)
	if err != nil {
		return fmt.Errorf("recreate failed; retain Workspace: %w", err)
	}
	if created.Workspace != old.Workspace || created.Name != old.Name || !created.CreatedAt.After(old.CreatedAt) {
		return fmt.Errorf("recreation identity mismatch")
	}
	return run("recreate", `test "$(cat /workspace/work.txt)" = uncommitted-work || exit 41
test ! -e /root/haco-environment-only
printf verified`)
}
