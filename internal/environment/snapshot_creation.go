package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type savedRuntimeCreator interface {
	CreateEnvironmentFromSnapshot(context.Context, core.EnvironmentRuntimeSpec, core.Snapshot, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
}

// Route by saved rootfs, never by the deleted source Base or the current default.
func (r *BaseRouter) CreateEnvironmentFromSnapshot(ctx context.Context, spec core.EnvironmentRuntimeSpec, saved core.Snapshot, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if r == nil || r.Router == nil || record == nil || saved.State != "ready" || spec.Base != "" || spec.TemporaryWorkspace {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	var root *core.SnapshotComponent
	for _, c := range saved.Components {
		if c.Role == "rootfs" {
			if root != nil || c.State != "verified" {
				return core.EnvironmentRuntime{}, core.ErrIncompatibleState
			}
			copy := c
			root = &copy
		}
	}
	if root == nil {
		return core.EnvironmentRuntime{}, core.ErrIncompatibleState
	}
	p, _, providerID, err := r.resolveWithID(root.NativeRef)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	creator, ok := p.(savedRuntimeCreator)
	if !ok {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	return routeCreationReceipt(providerID, record, func(receipt func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
		return creator.CreateEnvironmentFromSnapshot(ctx, spec, saved, receipt)
	})
}
