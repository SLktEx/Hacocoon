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

const defaultCleanupTimeout = 30 * time.Second

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
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
	PutWorkspaceLease(context.Context, core.WorkspaceLease) error
	DeleteWorkspaceLease(context.Context, string) error
}

type Service struct {
	runtime        environmentRuntime
	store          environmentStore
	provider       WorkspaceProvider
	now            func() time.Time
	cleanupTimeout time.Duration
}

func New(runtime environmentRuntime, store environmentStore) *Service {
	return NewWithProvider(runtime, store, NewExternalPathWorkspace())
}

func NewWithProvider(runtime environmentRuntime, store environmentStore, provider WorkspaceProvider) *Service {
	return &Service{
		runtime:        runtime,
		store:          store,
		provider:       provider,
		now:            time.Now,
		cleanupTimeout: defaultCleanupTimeout,
	}
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
	resources, err := core.ResolveResourceBudget(spec.Resources)
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
		State:         core.WorkspaceLeaseAcquiring,
		AcquiredAt:    s.now().UTC(),
	}
	if err := s.store.AcquireWorkspaceLease(ctx, lease); err != nil {
		return core.Environment{}, fmt.Errorf("acquire workspace lease: %w", err)
	}
	releaseLease := func() error {
		cleanupCtx, cancel := s.newCleanupContext(ctx)
		defer cancel()
		err := s.store.DeleteWorkspaceLease(cleanupCtx, name)
		if isNotFound(err) {
			return nil
		}
		return err
	}

	created, err := s.runtime.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{
		Name:          name,
		WorkspacePath: workspace.Path,
		ReadOnly:      mode == core.WorkspaceReadOnly,
		Base:          spec.Base,
		Resources:     resources,
	})
	if err != nil {
		if errors.Is(err, core.ErrRecoveryRequired) {
			return core.Environment{}, fmt.Errorf("create environment %q: %w", name, err)
		}
		releaseErr := releaseLease()
		return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, err), releaseErr)
	}
	if created.Resources == (core.ResourceBudget{}) && !core.ResourceBudgetHasFinite(resources) {
		created.Resources = resources
	}
	if created.Resources != resources {
		cleanupErr := s.deleteRuntimeForCleanup(ctx, created.Ref)
		return core.Environment{}, errors.Join(
			fmt.Errorf("provider returned resource budget different from requested effective budget: %w", core.ErrIncompatibleState),
			cleanupErr,
			releaseLease(),
		)
	}

	lease.RuntimeRef = created.Ref
	lease.State = core.WorkspaceLeaseActive
	if err := s.store.PutWorkspaceLease(ctx, lease); err != nil {
		cleanupErr := s.deleteRuntimeForCleanup(ctx, created.Ref)
		if cleanupErr == nil {
			return core.Environment{}, errors.Join(fmt.Errorf("persist runtime reference in workspace lease: %w", err), releaseLease())
		}
		return core.Environment{}, errors.Join(
			fmt.Errorf("persist runtime reference in workspace lease: %w", err),
			cleanupErr,
			core.ErrRecoveryRequired,
		)
	}

	environment := core.Environment{
		Name:       name,
		Workspace:  workspace,
		AccessMode: mode,
		Base:       created.Base,
		Resources:  resources,
		RuntimeRef: created.Ref,
		CreatedAt:  s.now().UTC(),
	}
	if err := s.store.PutEnvironment(ctx, environment); err != nil {
		cleanupErr := s.deleteRuntimeForCleanup(ctx, created.Ref)
		if cleanupErr == nil {
			return core.Environment{}, errors.Join(fmt.Errorf("persist environment: %w", err), releaseLease())
		}
		lease.State = core.WorkspaceLeaseCleanupRequired
		markCtx, cancel := s.newCleanupContext(ctx)
		markErr := s.store.PutWorkspaceLease(markCtx, lease)
		cancel()
		return core.Environment{}, errors.Join(
			fmt.Errorf("persist environment: %w", err),
			cleanupErr,
			markErr,
			core.ErrRecoveryRequired,
		)
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
	if err == nil {
		if err := s.runtime.DeleteEnvironment(ctx, environment.RuntimeRef); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete runtime %q: %w", environment.RuntimeRef, err)
		}
		if err := s.store.DeleteEnvironment(ctx, name); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete environment metadata %q: %w", name, err)
		}
		if err := s.store.DeleteWorkspaceLease(ctx, name); err != nil && !isNotFound(err) {
			return fmt.Errorf("release workspace lease for %q: %w", name, err)
		}
		return nil
	}
	if !isNotFound(err) {
		return err
	}

	lease, leaseErr := s.store.GetWorkspaceLease(ctx, name)
	if isNotFound(leaseErr) {
		return nil
	}
	if leaseErr != nil {
		return leaseErr
	}
	if lease.RuntimeRef == "" {
		return fmt.Errorf("workspace lease for %q has no runtime reference; refusing to reclaim without proof: %w", name, core.ErrRecoveryRequired)
	}
	if err := s.runtime.DeleteEnvironment(ctx, lease.RuntimeRef); err != nil && !isNotFound(err) {
		return fmt.Errorf("recover runtime %q: %w", lease.RuntimeRef, err)
	}
	if err := s.store.DeleteWorkspaceLease(ctx, name); err != nil && !isNotFound(err) {
		return fmt.Errorf("release recovered workspace lease for %q: %w", name, err)
	}
	return nil
}

func (s *Service) deleteRuntimeForCleanup(parent context.Context, ref string) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	if err := s.runtime.DeleteEnvironment(cleanupCtx, ref); err != nil && !isNotFound(err) {
		return fmt.Errorf("cleanup runtime %q: %w", ref, err)
	}
	return nil
}

func (s *Service) newCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), s.cleanupTimeout)
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
