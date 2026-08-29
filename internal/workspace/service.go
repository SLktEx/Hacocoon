package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var environmentNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

type environmentRuntime interface {
	CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error)
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	ShellEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
}

type environmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	PutEnvironment(context.Context, core.Environment) error
	DeleteEnvironment(context.Context, string) error
}

type Service struct {
	runtime environmentRuntime
	store   environmentStore
	now     func() time.Time
}

func New(runtime environmentRuntime, store environmentStore) *Service {
	return &Service{runtime: runtime, store: store, now: time.Now}
}

func (s *Service) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	name, err := validateEnvironmentName(spec.Name)
	if err != nil {
		return core.Environment{}, err
	}
	workspacePath, err := resolveWorkspace(spec.WorkspacePath)
	if err != nil {
		return core.Environment{}, err
	}
	if _, err := s.store.GetEnvironment(ctx, name); err == nil {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrAlreadyExists)
	} else if !isNotFound(err) {
		return core.Environment{}, err
	}

	created, err := s.runtime.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{
		Name:          name,
		WorkspacePath: workspacePath,
	})
	if err != nil {
		return core.Environment{}, fmt.Errorf("create environment %q: %w", name, err)
	}

	environment := core.Environment{
		Name:       name,
		Workspace:  core.Workspace{Path: workspacePath},
		RuntimeRef: created.Ref,
		CreatedAt:  s.now().UTC(),
	}
	if err := s.store.PutEnvironment(ctx, environment); err != nil {
		cleanupErr := s.runtime.DeleteEnvironment(context.WithoutCancel(ctx), created.Ref)
		if cleanupErr != nil {
			return core.Environment{}, fmt.Errorf("persist environment: %v; cleanup runtime %q: %w", err, created.Ref, cleanupErr)
		}
		return core.Environment{}, fmt.Errorf("persist environment: %w", err)
	}
	return environment, nil
}

func (s *Service) Exec(ctx context.Context, name string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if len(req.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, req)
}

func (s *Service) Shell(ctx context.Context, name string) error {
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.ShellEnvironment(ctx, environment.RuntimeRef)
}

func (s *Service) Delete(ctx context.Context, name string) error {
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	if err := s.runtime.DeleteEnvironment(ctx, environment.RuntimeRef); err != nil {
		return fmt.Errorf("delete runtime %q: %w", environment.RuntimeRef, err)
	}
	if err := s.store.DeleteEnvironment(ctx, name); err != nil {
		return fmt.Errorf("delete environment metadata %q: %w", name, err)
	}
	return nil
}

func validateEnvironmentName(name string) (string, error) {
	if !environmentNamePattern.MatchString(name) {
		return "", fmt.Errorf("environment name %q must use lowercase letters, digits, and internal hyphens: %w", name, core.ErrInvalidArgument)
	}
	return name, nil
}

func resolveWorkspace(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("workspace path is required: %w", core.ErrInvalidArgument)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat workspace %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory: %w", resolved, core.ErrInvalidArgument)
	}
	return filepath.Clean(resolved), nil
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
