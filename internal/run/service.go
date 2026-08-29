package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Spec struct {
	WorkspacePath string                   `json:"workspace_path"`
	AccessMode    core.WorkspaceAccessMode `json:"access_mode"`
	Argv          []string                 `json:"argv"`
}

type ExecutionResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type Result struct {
	Environment string          `json:"environment"`
	Execution   ExecutionResult `json:"execution"`
	CleanedUp   bool            `json:"cleaned_up"`
}

type environmentLifecycle interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	Delete(context.Context, string) error
}

type Service struct {
	environments environmentLifecycle
	newName      func() (string, error)
}

func New(environments environmentLifecycle) *Service {
	return &Service{environments: environments, newName: randomEnvironmentName}
}

func (s *Service) Run(ctx context.Context, spec Spec) (Result, error) {
	if s == nil || s.environments == nil || spec.WorkspacePath == "" || len(spec.Argv) == 0 {
		return Result{}, core.ErrInvalidArgument
	}
	name, err := s.newName()
	if err != nil {
		return Result{}, fmt.Errorf("allocate run environment name: %w", err)
	}
	environment, err := s.environments.Create(ctx, core.EnvironmentSpec{
		Name:          name,
		WorkspacePath: spec.WorkspacePath,
		AccessMode:    spec.AccessMode,
	})
	if err != nil {
		return Result{Environment: name}, fmt.Errorf("create ephemeral environment: %w", err)
	}

	result := Result{Environment: environment.Name}
	execution, execErr := s.environments.Exec(ctx, environment.Name, core.ExecutionRequest{Argv: append([]string(nil), spec.Argv...)})
	result.Execution = ExecutionResult{ExitCode: execution.ExitCode, Stdout: execution.Stdout, Stderr: execution.Stderr}
	cleanupErr := s.environments.Delete(context.WithoutCancel(ctx), environment.Name)
	result.CleanedUp = cleanupErr == nil

	if execErr != nil && cleanupErr != nil {
		return result, errors.Join(execErr, fmt.Errorf("cleanup ephemeral environment %q: %w", environment.Name, cleanupErr))
	}
	if execErr != nil {
		return result, execErr
	}
	if cleanupErr != nil {
		return result, fmt.Errorf("cleanup ephemeral environment %q: %w", environment.Name, cleanupErr)
	}
	return result, nil
}

func randomEnvironmentName() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "run-" + hex.EncodeToString(bytes[:]), nil
}
