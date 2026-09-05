package clientadapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type controllerClient interface {
	CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error)
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error)
	ForwardEnvironment(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	UnforwardEnvironment(context.Context, string, string) error
	PrepareEnvironmentSSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
	DeleteEnvironment(context.Context, string) error
}

type controllerEnvironmentService struct {
	client controllerClient
}

type controllerClientService struct {
	client controllerClient
}

// NewController opens the client-neutral adapter through the Physical Host
// controller. It is safe to use from trusted haco-host because it never
// initializes guest-local composition or requires raw Incus authority.
//
// InteractionBatch is intentionally not provided by this constructor yet;
// this slice exposes only Environment and client-connection lifecycle needed by
// product clients such as `haco open`.
func NewController() (*Adapter, error) {
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		return nil, translateError(controllerError(err))
	}
	return newControllerAdapter(client), nil
}

// NewControllerAt is equivalent to NewController but uses an explicit Unix
// socket path. It is useful for process tests and alternate trusted controller
// projections without widening provider authority.
func NewControllerAt(path string) (*Adapter, error) {
	client, err := controlapi.NewClient(path)
	if err != nil {
		return nil, translateError(controllerError(err))
	}
	return newControllerAdapter(client), nil
}

func newControllerAdapter(client controllerClient) *Adapter {
	if client == nil {
		return newAdapter(nil, nil, nil)
	}
	return newAdapter(
		controllerEnvironmentService{client: client},
		controllerClientService{client: client},
		nil,
	)
}

func (s controllerEnvironmentService) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if s.client == nil {
		return core.Environment{}, core.ErrInvalidArgument
	}
	environment, err := s.client.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{
		Name:          spec.Name,
		WorkspacePath: spec.WorkspacePath,
		AccessMode:    spec.AccessMode,
		Base:          spec.Base,
		Resources:     spec.Resources,
	})
	return environment, controllerError(err)
}

func (s controllerEnvironmentService) Delete(ctx context.Context, name string) error {
	if s.client == nil {
		return core.ErrInvalidArgument
	}
	return controllerError(s.client.DeleteEnvironment(ctx, name))
}

func (s controllerClientService) Status(ctx context.Context, name string) (core.EnvironmentStatus, error) {
	if s.client == nil {
		return core.EnvironmentStatus{}, core.ErrInvalidArgument
	}
	status, err := s.client.EnvironmentStatus(ctx, name)
	return status, controllerError(err)
}

func (s controllerClientService) Connections(ctx context.Context, name string) ([]core.ClientConnection, error) {
	if s.client == nil {
		return nil, core.ErrInvalidArgument
	}
	connections, err := s.client.EnvironmentConnections(ctx, name)
	return connections, controllerError(err)
}

func (s controllerClientService) Forward(ctx context.Context, name string, request core.LocalPortRequest) (core.ClientConnection, error) {
	if s.client == nil {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	connection, err := s.client.ForwardEnvironment(ctx, name, request)
	return connection, controllerError(err)
}

func (s controllerClientService) Unforward(ctx context.Context, name, connectionID string) error {
	if s.client == nil {
		return core.ErrInvalidArgument
	}
	return controllerError(s.client.UnforwardEnvironment(ctx, name, connectionID))
}

func (s controllerClientService) SSH(ctx context.Context, name string, request core.SSHAccessRequest) (core.ClientConnection, error) {
	if s.client == nil {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	connection, err := s.client.PrepareEnvironmentSSH(ctx, name, request)
	return connection, controllerError(err)
}

func controllerError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, control.ErrUnavailable) {
		return fmt.Errorf("controller unavailable: %v: %w", err, core.ErrRuntimeUnavailable)
	}
	var status *control.StatusError
	if !errors.As(err, &status) {
		return err
	}
	var sentinel error
	switch status.Code {
	case "invalid_argument":
		sentinel = core.ErrInvalidArgument
	case "not_found":
		sentinel = core.ErrNotFound
	case "already_exists":
		sentinel = core.ErrAlreadyExists
	case "unsupported":
		sentinel = core.ErrUnsupported
	case "unavailable":
		sentinel = core.ErrRuntimeUnavailable
	case "busy":
		sentinel = core.ErrWorkspaceBusy
	case "incompatible_state":
		sentinel = core.ErrIncompatibleState
	case "recovery_required":
		sentinel = core.ErrRecoveryRequired
	default:
		return err
	}
	return fmt.Errorf("controller %s: %s: %w", status.Code, status.Message, sentinel)
}
