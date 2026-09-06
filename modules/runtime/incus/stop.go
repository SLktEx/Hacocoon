package incus

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) StopEnvironment(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	status, err := r.InspectEnvironment(ctx, ref)
	if err != nil {
		return err
	}
	if status.State == core.EnvironmentStopped {
		return nil
	}
	if status.State != core.EnvironmentRunning {
		return core.ErrIncompatibleState
	}
	if err := r.Stop(ctx, ref); err != nil {
		return err
	}
	status, err = r.InspectEnvironment(ctx, ref)
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentStopped {
		return fmt.Errorf("Environment has not stopped: %w", core.ErrIncompatibleState)
	}
	return nil
}
