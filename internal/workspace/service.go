package workspace

import (
	"context"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const defaultCleanupTimeout = 30 * time.Second

type environmentRuntime interface {
	CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error)
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	DeleteEnvironment(context.Context, string) error
}

// environmentStore exposes lifecycle transitions rather than independent
// Environment/Workspace-lease mutations. The production store implements each
// transition atomically, so callers cannot publish an active lease before its
// ready Environment metadata, or remove one side of the aggregate while the
// other remains durable.
type environmentStore interface {
	LockLifecycle(context.Context, string, string) (func(), error)
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
	BeginEnvironmentCreate(context.Context, core.WorkspaceLease) error
	RecordEnvironmentRuntime(context.Context, core.WorkspaceLease) error
	CommitEnvironmentCreate(context.Context, core.Environment, core.WorkspaceLease) error
	MarkEnvironmentRecoveryRequired(context.Context, core.WorkspaceLease) error
	FinalizeEnvironmentDelete(context.Context, string) error
}

type Service struct {
	checkSnapshotName          func(context.Context, string) error
	deleteWorkspaceResource    func(context.Context, string, core.WorkspaceID) error
	environmentResources       EnvironmentResources
	selectEnvironmentResources func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error)
	defaultResource            func(context.Context, core.Workspace) (core.PersistentResource, error)
	runtime                    environmentRuntime
	store                      environmentStore
	provider                   WorkspaceProvider
	now                        func() time.Time
	cleanupTimeout             time.Duration
}

func (s *Service) ConfigureWorkspaceResourceDeletion(remove func(context.Context, string, core.WorkspaceID) error) {
	s.deleteWorkspaceResource = remove
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

// ConfigureDefaultResource installs an optional provider-neutral initializer.
// Configure once at composition time, before serving concurrent requests.
func (s *Service) ConfigureDefaultResource(resolve func(context.Context, core.Workspace) (core.PersistentResource, error)) {
	s.defaultResource = resolve
}

func (s *Service) ConfigureSnapshotNames(check func(context.Context, string) error) {
	s.checkSnapshotName = check
}
func (s *Service) LockResourceName(ctx context.Context, name string) (func(), error) {
	if core.ValidateEnvironmentName(name) != nil || len(name) > 41 {
		return nil, core.ErrInvalidArgument
	}
	return s.lockLifecycle(ctx, "resource-name", name)
}
