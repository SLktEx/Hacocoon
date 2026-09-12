package workspace

import (
	"context"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func (s *Service) Exec(ctx context.Context, name string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	return s.exec(ctx, name, req, "")
}

// ExecForWorkspace serializes execution with inverse lifecycle operations and
// rejects recycled names before the provider can receive saved project input.
func (s *Service) ExecForWorkspace(ctx context.Context, name string, workspace core.WorkspaceID, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if workspace == "" {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	return s.exec(ctx, name, req, workspace)
}

func (s *Service) exec(ctx context.Context, name string, req core.ExecutionRequest, workspace core.WorkspaceID) (result core.ExecutionResult, err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "exec_environment", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "executing environment command")
	defer func() {
		attrs := []any{
			"duration_ms", time.Since(started).Milliseconds(),
			"exit_code", result.ExitCode,
		}
		if err != nil {
			if workspace == "" {
				logger.ErrorContext(ctx, "environment command failed", append(attrs, "error", err)...)
			} else {
				// Saved input and backend diagnostics may contain project secrets.
				// The setup boundary owns the failure log; do not duplicate it.
				logger.DebugContext(ctx, "project command failed", attrs...)
			}
			return
		}
		logger.InfoContext(ctx, "environment command completed", attrs...)
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	if len(req.Argv) == 0 || len(req.Stdin) > core.MaxExecutionInputBytes {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if workspace != "" {
		unlock, lockErr := lockLifecycle(ctx, "environment", name)
		if lockErr != nil {
			return core.ExecutionResult{}, lockErr
		}
		defer unlock()
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if workspace != "" && environment.Workspace.ID != workspace {
		return core.ExecutionResult{}, core.ErrIncompatibleState
	}
	return s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, req)
}

func (s *Service) Shell(ctx context.Context, name string) (err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "shell_environment", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "opening environment shell")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "environment shell failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.InfoContext(ctx, "environment shell closed", "duration_ms", time.Since(started).Milliseconds())
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.ShellEnvironment(ctx, environment.RuntimeRef)
}
