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
	unlock, err := lockLifecycle(ctx, "environment", name)
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
