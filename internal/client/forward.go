package client

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"time"
)

type forwardStore interface {
	GetReadyEnvironment(context.Context, string) (core.Environment, error)
	EnvironmentInstance(context.Context, core.Environment) (string, error)
}
type tcpRuntime interface {
	DialEnvironmentTCP(context.Context, string, string, string, int) (net.Conn, error)
}

// PrepareTCPForward uses trusted catalog identity, never a client-selected
// runtime reference. No provider socket is opened during preparation.
func (s *Service) PrepareTCPForward(ctx context.Context, name, address string, port int) (core.EnvironmentTCPForward, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var target core.EnvironmentTCPForward
	if s == nil || !connectionIDPattern.MatchString(name) || !core.ValidForwardAddress(address, port) {
		return target, core.ErrInvalidArgument
	}
	store, ok := s.store.(forwardStore)
	if !ok {
		return target, core.ErrUnsupported
	}
	if _, ok := s.runtime.(tcpRuntime); !ok {
		return target, core.ErrUnsupported
	}
	env, err := store.GetReadyEnvironment(ctx, name)
	if err != nil {
		return target, err
	}
	if env.Name != name || env.RuntimeRef == "" {
		return target, core.ErrIncompatibleState
	}
	instance, err := store.EnvironmentInstance(ctx, env)
	if err != nil {
		return target, err
	}
	if !core.ValidEnvironmentInstanceID(instance) {
		return target, core.ErrIncompatibleState
	}
	observed, err := s.runtime.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return target, err
	}
	if observed.Absent || observed.State != core.EnvironmentRunning {
		return target, core.ErrIncompatibleState
	}
	return core.EnvironmentTCPForward{Environment: name, Instance: instance, Address: address, Port: port}, nil
}

func (s *Service) DialTCPForward(ctx context.Context, target core.EnvironmentTCPForward) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !core.ValidEnvironmentInstanceID(target.Instance) {
		return nil, core.ErrInvalidArgument
	}
	current, err := s.PrepareTCPForward(ctx, target.Environment, target.Address, target.Port)
	if err != nil {
		return nil, err
	}
	if current != target {
		return nil, core.ErrCapabilityStale
	}
	store := s.store.(forwardStore)
	env, err := store.GetReadyEnvironment(ctx, target.Environment)
	if err != nil {
		return nil, err
	}
	instance, err := store.EnvironmentInstance(ctx, env)
	if err != nil {
		return nil, err
	}
	if instance != target.Instance {
		return nil, core.ErrCapabilityStale
	}
	// The adapter pins and verifies the provider generation around opening its
	// namespace. A lifecycle race cannot retarget this socket to a replacement.
	return s.runtime.(tcpRuntime).DialEnvironmentTCP(ctx, env.RuntimeRef, instance, target.Address, target.Port)
}
