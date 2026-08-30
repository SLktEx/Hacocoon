package workloadbroker

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type workloadService interface {
	CreateWorkload(context.Context, core.WorkloadSpec) (core.Workload, error)
	ListWorkloads(context.Context, string) ([]core.Workload, error)
	ExecWorkload(context.Context, string, string, []string) (core.ExecutionResult, error)
	StopWorkload(context.Context, string, string) error
	DeleteWorkload(context.Context, string, string) error
	PullWorkloadImage(context.Context, string) error
}

type Manager struct {
	runtime   *incus.Runtime
	workloads workloadService

	mu        sync.Mutex
	listeners map[string]net.Listener
	wg        sync.WaitGroup
}

func New(runtime *incus.Runtime, workloads workloadService) (*Manager, error) {
	if runtime == nil || workloads == nil {
		return nil, core.ErrInvalidArgument
	}
	return &Manager{
		runtime:   runtime,
		workloads: workloads,
		listeners: make(map[string]net.Listener),
	}, nil
}

// EnsureListener creates one Host-owned Unix socket whose handlers are bound to
// exactly one Environment. A guest cannot select another Environment in the
// request payload because the server closure itself carries the identity.
func (m *Manager) EnsureListener(ctx context.Context, environment string) error {
	if m == nil || m.runtime == nil || m.workloads == nil {
		return core.ErrInvalidArgument
	}
	path, err := incus.WorkloadBrokerSocketPath(environment)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.listeners[environment]; ok {
		return nil
	}
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		return err
	}
	server := control.NewServer()
	if err := controlapi.RegisterBoundEnvironmentWorkloads(server, m.workloads, environment); err != nil {
		_ = listener.Close()
		return err
	}
	m.listeners[environment] = listener
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		serveErr := server.Serve(ctx, listener)
		m.mu.Lock()
		if current, ok := m.listeners[environment]; ok && current == listener {
			delete(m.listeners, environment)
		}
		m.mu.Unlock()
		if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
			logging.Root().ErrorContext(ctx, "Environment workload broker stopped",
				"component", "control",
				"environment_id", environment,
				"error", serveErr,
			)
		}
	}()
	return nil
}

// ReconcileExisting restores both the Host listener and the guest-side proxy /
// nerdctl shim for an Environment persisted before a controller restart or an
// upgrade to the Incus-native workload path.
func (m *Manager) ReconcileExisting(ctx context.Context, environment core.Environment) error {
	if err := m.EnsureListener(ctx, environment.Name); err != nil {
		return err
	}
	if err := m.runtime.EnsureEnvironmentWorkloadIntegration(ctx, environment.Name, environment.RuntimeRef); err != nil {
		_ = m.Remove(environment.Name)
		return err
	}
	return nil
}

func (m *Manager) Remove(environment string) error {
	if m == nil {
		return core.ErrInvalidArgument
	}
	m.mu.Lock()
	listener := m.listeners[environment]
	delete(m.listeners, environment)
	m.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	listeners := make([]net.Listener, 0, len(m.listeners))
	for environment, listener := range m.listeners {
		delete(m.listeners, environment)
		listeners = append(listeners, listener)
	}
	m.mu.Unlock()
	var errs []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
	}
	m.wg.Wait()
	return errors.Join(errs...)
}

// EnvironmentService makes broker listener lifecycle transactional with the
// existing Workspace service while leaving Environment persistence semantics in
// the original service.
type EnvironmentService struct {
	base    *workspaceapp.Service
	brokers *Manager
}

func NewEnvironmentService(base *workspaceapp.Service, brokers *Manager) (*EnvironmentService, error) {
	if base == nil || brokers == nil {
		return nil, core.ErrInvalidArgument
	}
	return &EnvironmentService{base: base, brokers: brokers}, nil
}

func (s *EnvironmentService) Reconcile(ctx context.Context) error {
	environments, err := s.base.List(ctx)
	if err != nil {
		return err
	}
	for _, environment := range environments {
		if err := s.brokers.ReconcileExisting(ctx, environment); err != nil {
			return err
		}
	}
	return nil
}

func (s *EnvironmentService) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if err := s.brokers.EnsureListener(ctx, spec.Name); err != nil {
		return core.Environment{}, err
	}
	environment, err := s.base.Create(ctx, spec)
	if err != nil {
		_ = s.brokers.Remove(spec.Name)
		return core.Environment{}, err
	}
	return environment, nil
}

func (s *EnvironmentService) List(ctx context.Context) ([]core.Environment, error) {
	return s.base.List(ctx)
}

func (s *EnvironmentService) Exec(ctx context.Context, environment string, request core.ExecutionRequest) (core.ExecutionResult, error) {
	return s.base.Exec(ctx, environment, request)
}

func (s *EnvironmentService) PrepareShellStream(ctx context.Context, environment string) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	return s.base.PrepareShellStream(ctx, environment)
}

func (s *EnvironmentService) Delete(ctx context.Context, environment string) error {
	if err := s.base.Delete(ctx, environment); err != nil {
		return err
	}
	return s.brokers.Remove(environment)
}
