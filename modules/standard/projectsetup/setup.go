package projectsetup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

type Environments interface {
	Get(context.Context, string) (core.Environment, error)
	StartForWorkspace(context.Context, string, core.WorkspaceID) error
	ExecForWorkspace(context.Context, string, core.WorkspaceID, core.ExecutionRequest) (core.ExecutionResult, error)
}
type Service struct {
	Root         string
	Environments Environments
}
type Result struct {
	FailureStage string               `json:"failure_stage,omitempty"`
	Environment  string               `json:"environment"`
	Workspace    core.WorkspaceID     `json:"workspace_id"`
	Applied      bool                 `json:"applied"`
	Cleared      bool                 `json:"cleared"`
	Execution    core.ExecutionResult `json:"execution"`
}

// Apply uses a Workspace-scoped snapshot but executes only in the explicitly
// selected Environment. Its Host counterpart uses a distinct store and adapter.
func (s *Service) Apply(ctx context.Context, name string, update recipes.Update) (result Result, err error) {
	result.FailureStage = "validate"
	defer func() {
		if err == nil {
			result.FailureStage = ""
		}
	}()
	if s == nil || s.Environments == nil || !filepath.IsAbs(s.Root) {
		return result, core.ErrInvalidArgument
	}
	if err = update.Validate(); err != nil || update.Reapply || update.ResultOnly {
		return result, core.ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	result.FailureStage = "lookup"
	environment, err := s.Environments.Get(ctx, name)
	if err != nil {
		return result, err
	}
	if environment.Name != name || environment.Workspace.ID == "" || core.IsTemporaryWorkspacePath(environment.Workspace.Path) {
		return result, core.ErrInvalidArgument
	}
	result.Environment = name
	result.Workspace = environment.Workspace.ID
	result.Cleared = update.Clear
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(environment.Workspace.ID)))
	result.FailureStage = "recipe"
	storage := recipes.Service{Root: filepath.Join(s.Root, identity), Execute: func(ctx context.Context, script []byte) error {
		result.FailureStage = "start"
		if err := s.Environments.StartForWorkspace(ctx, name, environment.Workspace.ID); err != nil {
			return err
		}
		seconds := int64(14 * 60)
		if deadline, ok := ctx.Deadline(); ok {
			remaining := int64((time.Until(deadline) - 10*time.Second) / time.Second)
			if remaining < seconds {
				seconds = remaining
			}
		}
		if seconds < 1 {
			return context.DeadlineExceeded
		}
		argv := []string{"/usr/bin/systemd-run", "--unit=hacocoon-project-setup", "--collect", "--wait", "--pipe", "--service-type=exec",
			"--working-directory=/workspace", "--property=KillMode=control-group", "--property=TimeoutStopSec=5s",
			fmt.Sprintf("--property=RuntimeMaxSec=%ds", seconds)}
		// Copy only the Environment's public proxy settings into its transient unit.
		// No Host credentials or controller routing variables are forwarded.
		for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
			argv = append(argv, "--setenv="+key)
		}
		argv = append(argv, "/bin/bash", "-se")
		result.FailureStage = "execute"
		execution, err := s.Environments.ExecForWorkspace(ctx, name, environment.Workspace.ID, core.ExecutionRequest{
			WorkingDirectory: "/workspace", Argv: argv, Stdin: script,
		})
		result.Execution = execution
		result.Applied = true
		if err != nil {
			return err
		}
		if execution.ExitCode != 0 {
			result.FailureStage = "script"
			return fmt.Errorf("project setup exited with status %d", execution.ExitCode)
		}
		return nil
	}}
	err = storage.Apply(ctx, update)
	return result, err
}
