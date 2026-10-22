package workspace

import (
	"context"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type processRuntime interface {
	ExecEnvironmentStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

// ExecRunStream pins execution to the owned creation under the same lifecycle
// lock used by deletion. Long-lived input cannot be redirected to a replacement.
func (s *Service) ExecRunStream(ctx context.Context, name, instance string, request core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	if s == nil || !core.ValidEnvironmentInstanceID(instance) || len(request.Argv) == 0 || stdin == nil || stdout == nil || stderr == nil {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if err := core.ValidateProcessRequest(request); err != nil {
		return core.ExecutionResult{}, err
	}
	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer unlock()
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !lease.Ephemeral || lease.InstanceID != instance || lease.State != core.WorkspaceLeaseActive {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if env.RuntimeRef != lease.RuntimeRef {
		return core.ExecutionResult{}, core.ErrIncompatibleState
	}
	runtime, ok := s.runtime.(processRuntime)
	if !ok {
		return core.ExecutionResult{}, core.ErrUnsupported
	}
	return runtime.ExecEnvironmentStream(ctx, env.RuntimeRef, request, stdin, stdout, stderr)
}

// ExecStream executes only in an existing, running Environment. The lifecycle
// lock pins the exact ownership relation until the stream has ended.
func (s *Service) ExecStream(ctx context.Context, name string, request core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	if s == nil || stdin == nil || stdout == nil || stderr == nil {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if err := core.ValidateProcessRequest(request); err != nil {
		return core.ExecutionResult{}, err
	}
	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer unlock()
	env, err := s.runningEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	runtime, ok := s.runtime.(processRuntime)
	if !ok {
		return core.ExecutionResult{}, core.ErrUnsupported
	}
	return runtime.ExecEnvironmentStream(ctx, env.RuntimeRef, request, stdin, stdout, stderr)
}

// Called with the lifecycle lock held; observation never changes runtime state.
func (s *Service) runningEnvironment(ctx context.Context, name string) (core.Environment, error) {
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.Environment{}, err
	}
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return core.Environment{}, err
	}
	if lease.Ephemeral || !core.ValidEnvironmentInstanceID(lease.InstanceID) || !lease.MatchesEnvironment(env) {
		return core.Environment{}, core.ErrIncompatibleState
	}
	observer, ok := s.runtime.(interface {
		InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
	})
	if !ok {
		return core.Environment{}, core.ErrUnsupported
	}
	status, err := observer.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return core.Environment{}, err
	}
	if status.State != core.EnvironmentRunning {
		return core.Environment{}, core.ErrIncompatibleState
	}
	return env, nil
}
func (s *Service) ExecUser(ctx context.Context, name string, request core.ExecutionRequest) (core.ExecutionResult, error) {
	if core.ValidateEnvironmentName(name) != nil || core.ValidateProcessRequest(core.ProcessRequest{Argv: request.Argv}) != nil || len(request.Stdin) > core.MaxExecutionInputBytes {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer unlock()
	env, err := s.runningEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return s.runtime.ExecEnvironment(ctx, env.RuntimeRef, request)
}
