package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

type receiptCreator interface {
	CreateEnvironmentWithReceipt(context.Context, core.EnvironmentRuntimeSpec, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
}

func (r *BaseRouter) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if r == nil || r.Router == nil || record == nil {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	p, err := r.provider(r.defaultProvider)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err = validateTemporaryProvider(p, spec); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if staged, ok := p.(receiptCreator); ok {
		return routeCreationReceipt(r.defaultProvider, record, func(receipt func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
			return staged.CreateEnvironmentWithReceipt(ctx, spec, receipt)
		})
	}
	// Existing non-staged providers still return a complete runtime. They do not
	// gain an earlier receipt guarantee merely by passing through the router.
	v, err := r.CreateEnvironment(ctx, spec)
	if err != nil {
		return v, err
	}
	return v, record(v)
}

// A single receipt protocol qualifies native ownership before any caller sees it.
func routeCreationReceipt(providerID string, record func(core.EnvironmentRuntime) error, create func(func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)) (core.EnvironmentRuntime, error) {
	qualify := func(v core.EnvironmentRuntime) (core.EnvironmentRuntime, error) {
		if strings.TrimSpace(v.Ref) == "" || strings.HasPrefix(v.Ref, refPrefix) {
			return core.EnvironmentRuntime{}, core.ErrIncompatibleState
		}
		v.Ref = encodeRouteRef(providerID, v.Ref)
		return v, nil
	}

	called := false
	var owned core.EnvironmentRuntime
	var receiptErr error
	v, err := create(func(native core.EnvironmentRuntime) error {
		if called {
			receiptErr = core.ErrIncompatibleState
			return receiptErr
		}
		called = true
		owned, receiptErr = qualify(native)
		if receiptErr == nil {
			receiptErr = record(owned)
		}
		return receiptErr
	})
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if !called || receiptErr != nil {
		return core.EnvironmentRuntime{}, errors.Join(core.ErrRecoveryRequired, receiptErr, core.ErrIncompatibleState)
	}
	routed, err := qualify(v)
	if err != nil || routed.Ref != owned.Ref {
		return core.EnvironmentRuntime{}, errors.Join(core.ErrRecoveryRequired, core.ErrCapabilityStale, err)
	}
	return routed, nil
}
