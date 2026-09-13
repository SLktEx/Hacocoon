package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// WithClientAccess serializes access preparation and socket acquisition with
// canonical lifecycle mutations. No long-lived session holds this lock.
func (s *Service) WithClientAccess(ctx context.Context, name string, expected *core.StreamTarget, resume bool, authorize func(core.Environment, string) error, operation func(core.Environment, string) error) error {
	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	unlock, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.checkSnapshotIdle(ctx, name); err != nil {
		return err
	}
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil || lease.State != core.WorkspaceLeaseActive || !lease.MatchesEnvironment(env) || lease.Owner != env.Name || lease.SnapshotSource != "" || lease.AcquiredAt.IsZero() || !lease.AcquiredAt.Equal(env.CreatedAt) || !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return core.ErrRecoveryRequired
	}
	if expected != nil && (!expected.Valid() || expected.Environment != env.Name || expected.Instance != lease.InstanceID || expected.Workspace != env.Workspace.ID || expected.AccessMode != env.AccessMode) {
		return core.ErrCapabilityStale
	}
	runtime, ok := s.runtime.(interface {
		VerifyEnvironmentIdentity(context.Context, string, string) error
		InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
		StartEnvironment(context.Context, string) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	if err := runtime.VerifyEnvironmentIdentity(ctx, env.RuntimeRef, lease.InstanceID); err != nil {
		return err
	}
	if authorize != nil {
		if err := authorize(env, lease.InstanceID); err != nil {
			return err
		}
	}
	state, err := runtime.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return err
	}
	if state.Absent {
		return core.ErrRecoveryRequired
	}
	if state.State == core.EnvironmentStopped && resume {
		if err := runtime.StartEnvironment(ctx, env.RuntimeRef); err != nil {
			return err
		}
		state, err = runtime.InspectEnvironment(ctx, env.RuntimeRef)
		if err != nil {
			return err
		}
	}
	if state.State != core.EnvironmentRunning {
		return core.ErrIncompatibleState
	}
	return operation(env, lease.InstanceID)
}
