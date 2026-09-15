package incus

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Receipt-free direct provider callers own failure cleanup. The canonical
// receipt path delegates to Workspace instead. The selected deletion method
// retains provider-specific obligations, including Sandbox source guards.
func (r *Runtime) cleanupFailedEnvironment(parent context.Context, ref string, cause error, deleteEnvironment func(context.Context, string) error) (core.EnvironmentRuntime, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), r.cleanupTimeout)
	defer cancel()
	err := deleteEnvironment(ctx, ref)
	if core.EnvironmentDeletionComplete(err) {
		return core.EnvironmentRuntime{}, cause
	}
	return core.EnvironmentRuntime{}, errors.Join(cause, fmt.Errorf("cleanup Incus environment %s: %w", ref, err), core.ErrRecoveryRequired)
}
