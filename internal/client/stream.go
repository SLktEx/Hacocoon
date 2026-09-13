package client

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"time"
)

type accessLifecycle interface {
	WithClientAccess(context.Context, string, *core.StreamTarget, bool, func(core.Environment, string) error, func(core.Environment, string) error) error
}

func NewWithLifecycle(runtime accessRuntime, store environmentStore, lifecycle accessLifecycle) *Service {
	s := New(runtime, store)
	s.lifecycle = lifecycle
	return s
}

func (s *Service) DialStream(ctx context.Context, target core.StreamTarget) (net.Conn, error) {
	if !target.Valid() || !connectionIDPattern.MatchString(target.Environment) || !connectionIDPattern.MatchString(target.Grant) {
		return nil, core.ErrInvalidArgument
	}
	if s.lifecycle == nil {
		return nil, core.ErrUnsupported
	}
	dialer, ok := s.runtime.(interface {
		DialEnvironmentNetwork(context.Context, string, string, string, int) (net.Conn, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var conn net.Conn
	err := s.lifecycle.WithClientAccess(ctx, target.Environment, &target, true, func(env core.Environment, instance string) error {
		grants, err := s.runtime.ListClientConnections(ctx, env.RuntimeRef)
		if err != nil {
			return err
		}
		found := false
		for _, grant := range grants {
			if grant.ID == target.Grant && grant.Kind == "ssh" && grant.Port == 0 && grant.TargetPort == 22 {
				found = true
			}
		}
		if !found {
			return core.ErrPolicyDenied
		}
		return nil
	}, func(env core.Environment, instance string) error {
		// Only the named service is supported. There is no caller-selected Host
		// address, provider reference, or arbitrary port.
		for {
			var err error
			conn, err = dialer.DialEnvironmentNetwork(ctx, env.RuntimeRef, instance, "tcp", 22)
			if err == nil {
				conn = s.trackStream(streamBinding{env.Name, instance, target.Grant}, conn)
				return nil
			}
			// sshd may start after the guest reports running. Retry only refusal.
			if !isConnectionRefused(err) {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	})
	return conn, err
}
