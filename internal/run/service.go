package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultCleanupTimeout = 30 * time.Second

type Spec struct {
	WorkspacePath string                   `json:"workspace_path"`
	AccessMode    core.WorkspaceAccessMode `json:"access_mode"`
	Argv          []string                 `json:"argv"`
}

type ExecutionResult struct {
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	StdoutBytes     int64  `json:"stdout_bytes"`
	StderrBytes     int64  `json:"stderr_bytes"`
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
	environments   environmentLifecycle
	newName        func() (string, error)
	cleanupTimeout time.Duration
}

func New(environments environmentLifecycle) *Service {
	return &Service{
		environments:   environments,
		newName:        randomEnvironmentName,
		cleanupTimeout: defaultCleanupTimeout,
	}
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
	_, stdoutMarker, stdoutMarkerBytes := host.DecodeCapturedOutput(execution.Stdout)
	_, stderrMarker, stderrMarkerBytes := host.DecodeCapturedOutput(execution.Stderr)
	result.Execution = ExecutionResult{
		ExitCode:        execution.ExitCode,
		Stdout:          execution.Stdout,
		Stderr:          execution.Stderr,
		StdoutTruncated: execution.StdoutTruncated || stdoutMarker,
		StderrTruncated: execution.StderrTruncated || stderrMarker,
		StdoutBytes:     outputByteCount(execution.StdoutBytes, stdoutMarkerBytes),
		StderrBytes:     outputByteCount(execution.StderrBytes, stderrMarkerBytes),
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	cleanupErr := s.environments.Delete(cleanupCtx, environment.Name)
	cancel()
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

func outputByteCount(explicit, decoded int64) int64 {
	if explicit > 0 {
		return explicit
	}
	return decoded
}

func randomEnvironmentName() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "run-" + hex.EncodeToString(bytes[:]), nil
}
