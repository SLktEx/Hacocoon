package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

type archiveRuntimeCreator interface {
	CreateEnvironmentFromArchive(context.Context, core.EnvironmentRuntimeSpec, io.ReadSeeker, string, int64, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
}

// Incus unified archives are explicitly routed to Incus. The current default
// provider and source Base metadata cannot select a different interpreter.
func (r *BaseRouter) CreateEnvironmentFromArchive(ctx context.Context, spec core.EnvironmentRuntimeSpec, source io.ReadSeeker, privateRoot string, limit int64, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if r == nil || r.Router == nil || source == nil || record == nil || limit <= 0 || spec.Base != "" || spec.TemporaryWorkspace || spec.ResourceMaintenance {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	provider, err := r.provider(ProviderIncus)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	creator, ok := provider.(archiveRuntimeCreator)
	if !ok {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	return routeCreationReceipt(ProviderIncus, record, func(receipt func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
		return creator.CreateEnvironmentFromArchive(ctx, spec, source, privateRoot, limit, receipt)
	})
}
