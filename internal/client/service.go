package client

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var connectionIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

type environmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
}

type accessRuntime interface {
	InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
	ListClientConnections(context.Context, string) ([]core.ClientConnection, error)
	ForwardLocalPort(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	RemoveClientConnection(context.Context, string, string) error
	PrepareSSHAccess(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
	RevokeSSHAccess(context.Context, string, string) error
}

type Service struct {
	runtime accessRuntime
	store   environmentStore
}

func New(runtime accessRuntime, store environmentStore) *Service {
	return &Service{runtime: runtime, store: store}
}

func (s *Service) Status(ctx context.Context, name string) (core.EnvironmentStatus, error) {
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.EnvironmentStatus{}, err
	}
	observed, err := s.runtime.InspectEnvironment(ctx, environment.RuntimeRef)
	if err != nil {
		return core.EnvironmentStatus{}, fmt.Errorf("inspect environment %q: %w", name, err)
	}
	return core.EnvironmentStatus{Environment: environment, State: observed.State}, nil
}

func (s *Service) Connections(ctx context.Context, name string) ([]core.ClientConnection, error) {
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return nil, err
	}
	connections, err := s.runtime.ListClientConnections(ctx, environment.RuntimeRef)
	if err != nil {
		return nil, fmt.Errorf("list client connections for %q: %w", name, err)
	}
	return connections, nil
}

func (s *Service) Forward(ctx context.Context, name string, req core.LocalPortRequest) (core.ClientConnection, error) {
	normalized, err := normalizePortRequest(req)
	if err != nil {
		return core.ClientConnection{}, err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ClientConnection{}, err
	}
	return s.runtime.ForwardLocalPort(ctx, environment.RuntimeRef, normalized)
}

func (s *Service) Unforward(ctx context.Context, name, connectionID string) error {
	if !connectionIDPattern.MatchString(connectionID) {
		return fmt.Errorf("connection id %q: %w", connectionID, core.ErrInvalidArgument)
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	if strings.HasPrefix(connectionID, "ssh-") {
		return s.runtime.RevokeSSHAccess(ctx, environment.RuntimeRef, connectionID)
	}
	return s.runtime.RemoveClientConnection(ctx, environment.RuntimeRef, connectionID)
}

func (s *Service) SSH(ctx context.Context, name string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	if req.HostPort < 1 || req.HostPort > 65535 {
		return core.ClientConnection{}, fmt.Errorf("SSH host port %d: %w", req.HostPort, core.ErrInvalidArgument)
	}
	key, err := normalizePublicKey(req.PublicKey)
	if err != nil {
		return core.ClientConnection{}, err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ClientConnection{}, err
	}
	req.PublicKey = key
	return s.runtime.PrepareSSHAccess(ctx, environment.RuntimeRef, req)
}

func normalizePortRequest(req core.LocalPortRequest) (core.LocalPortRequest, error) {
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Protocol != "tcp" {
		return core.LocalPortRequest{}, fmt.Errorf("protocol %q: %w", req.Protocol, core.ErrUnsupported)
	}
	if req.HostPort < 1 || req.HostPort > 65535 || req.TargetPort < 1 || req.TargetPort > 65535 {
		return core.LocalPortRequest{}, fmt.Errorf("ports host=%d target=%d: %w", req.HostPort, req.TargetPort, core.ErrInvalidArgument)
	}
	return req, nil
}
