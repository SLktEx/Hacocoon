package client

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

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
	streamMu  sync.Mutex
	streams   map[streamBinding]map[*ownedStream]struct{}
	lifecycle accessLifecycle
	runtime   accessRuntime
	store     environmentStore
}

func New(runtime accessRuntime, store environmentStore) *Service {
	return &Service{runtime: runtime, store: store}
}

func (s *Service) Status(ctx context.Context, name string) (core.EnvironmentStatus, error) {
	environment, err := s.environmentForStatus(ctx, name)
	if err != nil {
		return core.EnvironmentStatus{}, err
	}
	observed, err := s.runtime.InspectEnvironment(ctx, environment.RuntimeRef)
	if err != nil {
		return core.EnvironmentStatus{}, fmt.Errorf("inspect environment %q: %w", name, err)
	}
	if observed.Absent {
		return core.EnvironmentStatus{}, fmt.Errorf("environment %q has retained ownership but no runtime: %w", name, core.ErrRecoveryRequired)
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
	if s.lifecycle != nil {
		store, ok := s.store.(interface {
			EnvironmentInstance(context.Context, core.Environment) (string, error)
		})
		if !ok {
			return nil, core.ErrUnsupported
		}
		instance, err := store.EnvironmentInstance(ctx, environment)
		if err != nil {
			return nil, err
		}
		for i := range connections {
			if connections[i].Kind == "ssh" && connections[i].Port == 0 {
				connections[i].Target = &core.StreamTarget{Environment: name, Instance: instance, Workspace: environment.Workspace.ID, AccessMode: environment.AccessMode, Service: "ssh", Grant: connections[i].ID}
			}
		}
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
		if s.lifecycle != nil {
			return s.lifecycle.WithClientAccess(ctx, name, nil, true, nil, func(env core.Environment, instance string) error {
				s.closeStreams(streamBinding{env.Name, instance, connectionID})
				return s.runtime.RevokeSSHAccess(ctx, env.RuntimeRef, connectionID)
			})
		}
		return s.runtime.RevokeSSHAccess(ctx, environment.RuntimeRef, connectionID)
	}
	return s.runtime.RemoveClientConnection(ctx, environment.RuntimeRef, connectionID)
}

func (s *Service) SSH(ctx context.Context, name string, req core.SSHAccessRequest) (core.ClientConnection, error) {

	key, err := normalizePublicKey(req.PublicKey)
	if err != nil {
		return core.ClientConnection{}, err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ClientConnection{}, err
	}
	req.PublicKey = key
	if s.lifecycle != nil {
		var conn core.ClientConnection
		err := s.lifecycle.WithClientAccess(ctx, name, nil, true, nil, func(env core.Environment, instance string) error {
			var err error
			if migration, ok := s.runtime.(interface {
				MigrateSSHAccess(context.Context, string) error
			}); ok {
				if err = migration.MigrateSSHAccess(ctx, env.RuntimeRef); err != nil {
					return err
				}
			}
			conn, err = s.runtime.PrepareSSHAccess(ctx, env.RuntimeRef, req)
			if err == nil {
				conn.Target = &core.StreamTarget{Environment: name, Instance: instance, Workspace: env.Workspace.ID, AccessMode: env.AccessMode, Service: "ssh", Grant: conn.ID}
			}
			return err
		})
		return conn, err
	}
	return s.runtime.PrepareSSHAccess(ctx, environment.RuntimeRef, req)
}

func normalizePortRequest(req core.LocalPortRequest) (core.LocalPortRequest, error) {
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Protocol != "tcp" && req.Protocol != "udp" {
		return core.LocalPortRequest{}, fmt.Errorf("protocol %q: %w", req.Protocol, core.ErrUnsupported)
	}
	if req.HostPort < 0 || req.HostPort > 65535 || req.TargetPort < 1 || req.TargetPort > 65535 {
		return core.LocalPortRequest{}, fmt.Errorf("ports host=%d target=%d: %w", req.HostPort, req.TargetPort, core.ErrInvalidArgument)
	}
	return req, nil
}

// Production status uses one catalog observation of metadata and its lease.
// Minimal client stores retain their existing interface for alternate adapters.
func (s *Service) environmentForStatus(ctx context.Context, name string) (core.Environment, error) {
	if store, ok := s.store.(interface {
		GetReadyEnvironment(context.Context, string) (core.Environment, error)
	}); ok {
		return store.GetReadyEnvironment(ctx, name)
	}
	return s.store.GetEnvironment(ctx, name)
}
