package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	AcquireWorkspaceLease(context.Context, core.WorkspaceLease) error
	DeleteWorkspaceLease(context.Context, string) error
}

type Service struct {
	runtime  environmentRuntime
	store    environmentStore
	provider WorkspaceProvider
	now      func() time.Time
}

func New(runtime environmentRuntime, store environmentStore) *Service {
	return NewWithProvider(runtime, store, NewExternalPathWorkspace())
}

func NewWithProvider(runtime environmentRuntime, store environmentStore, provider WorkspaceProvider) *Service {
	return &Service{runtime: runtime, store: store, provider: provider, now: time.Now}
}

func (s *Service) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	name, err := validateEnvironmentName(spec.Name)
	if err != nil {
		return core.Environment{}, err
	}
	mode, err := normalizeAccessMode(spec.AccessMode)
	if err != nil {
		return core.Environment{}, err
	}
	workspace, err := s.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath})
	if err != nil {
		return core.Environment{}, err
	}
	unlock, err := lockWorkspace(ctx, workspace.ID)
	if err != nil {
		return core.Environment{}, fmt.Errorf("lock workspace: %w", err)
	}
	defer unlock()
	if _, err := s.store.GetEnvironment(ctx, name); err == nil {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrAlreadyExists)
	} else if !isNotFound(err) {
		return core.Environment{}, err
	}

	lease := core.WorkspaceLease{
		WorkspaceID:   workspace.ID,
		SourcePath:    workspace.Path,
		EnvironmentID: name,
		AccessMode:    mode,
		Owner:         name,
		AcquiredAt:    s.now().UTC(),
	}
	if err := s.store.AcquireWorkspaceLease(ctx, lease); err != nil {
		return core.Environment{}, fmt.Errorf("acquire workspace lease: %w", err)
	}
	releaseLease := func() error {
		err := s.store.DeleteWorkspaceLease(context.WithoutCancel(ctx), name)
		if isNotFound(err) {
			return nil
		}
		return err
	}

	created, err := s.runtime.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{
		Name:          name,
		WorkspacePath: workspace.Path,
		ReadOnly:      mode == core.WorkspaceReadOnly,
	})
	if err != nil {
		releaseErr := releaseLease()
		return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, err), releaseErr)
	}

	environment := core.Environment{
		Name:       name,
		Workspace:  workspace,
		AccessMode: mode,
		RuntimeRef: created.Ref,
		CreatedAt:  s.now().UTC(),
	}
	if err := s.store.PutEnvironment(ctx, environment); err != nil {
		cleanupErr := s.runtime.DeleteEnvironment(context.WithoutCancel(ctx), created.Ref)
		releaseErr := releaseLease()
		return core.Environment{}, errors.Join(fmt.Errorf("persist environment: %w", err), cleanupErr, releaseErr)
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
	if err := s.store.DeleteWorkspaceLease(ctx, name); err != nil && !isNotFound(err) {
		return fmt.Errorf("release workspace lease for %q: %w", name, err)
	}
	return nil
}

func validateEnvironmentName(name string) (string, error) {
	if !environmentNamePattern.MatchString(name) {
		return "", fmt.Errorf("environment name %q must use lowercase letters, digits, and internal hyphens: %w", name, core.ErrInvalidArgument)
	}
	return name, nil
}

func normalizeAccessMode(mode core.WorkspaceAccessMode) (core.WorkspaceAccessMode, error) {
	if mode == "" {
		return core.WorkspaceReadWrite, nil
	}
	switch mode {
	case core.WorkspaceReadOnly, core.WorkspaceReadWrite:
		return mode, nil
	default:
		return "", fmt.Errorf("workspace access mode %q: %w", mode, core.ErrInvalidArgument)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
