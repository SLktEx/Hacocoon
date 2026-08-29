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
	ForwardLocalPort(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	RemoveClientConnection(context.Context, string, string) error
	PrepareSSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
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

func (s *Service) Forward(ctx context.Context, name string, req core.LocalPortRequest) (core.ClientConnection, error) {
	if err := validatePortRequest(req); err != nil {
		return core.ClientConnection{}, err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ClientConnection{}, err
	}
	return s.runtime.ForwardLocalPort(ctx, environment.RuntimeRef, req)
}

func (s *Service) Unforward(ctx context.Context, name, connectionID string) error {
	if !connectionIDPattern.MatchString(connectionID) {
		return fmt.Errorf("connection id %q: %w", connectionID, core.ErrInvalidArgument)
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.RemoveClientConnection(ctx, environment.RuntimeRef, connectionID)
}

func (s *Service) SSH(ctx context.Context, name string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	if req.HostPort < 1 || req.HostPort > 65535 {
		return core.ClientConnection{}, fmt.Errorf("SSH host port %d: %w", req.HostPort, core.ErrInvalidArgument)
	}
	key := strings.TrimSpace(req.PublicKey)
	if key == "" || strings.ContainsAny(key, "\r\n") || !looksLikePublicKey(key) {
		return core.ClientConnection{}, fmt.Errorf("SSH public key: %w", core.ErrInvalidArgument)
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ClientConnection{}, err
	}
	req.PublicKey = key
	return s.runtime.PrepareSSH(ctx, environment.RuntimeRef, req)
}

func validatePortRequest(req core.LocalPortRequest) error {
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Protocol != "tcp" {
		return fmt.Errorf("protocol %q: %w", req.Protocol, core.ErrUnsupported)
	}
	if req.HostPort < 1 || req.HostPort > 65535 || req.TargetPort < 1 || req.TargetPort > 65535 {
		return fmt.Errorf("ports host=%d target=%d: %w", req.HostPort, req.TargetPort, core.ErrInvalidArgument)
	}
	return nil
}

func looksLikePublicKey(key string) bool {
	return strings.HasPrefix(key, "ssh-") || strings.HasPrefix(key, "ecdsa-") || strings.HasPrefix(key, "sk-ssh-") || strings.HasPrefix(key, "sk-ecdsa-")
}
