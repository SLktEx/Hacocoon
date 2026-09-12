package aws

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

// GuestSource is supplied by a trusted source resolver, never decoded from a
// workload request. Reusing an Environment name is not reusing its authority.
type GuestSource struct{ Environment, Instance string }

func (b *Broker) ListFromGuest(ctx context.Context, source GuestSource, s ListSpec) (core.CapabilityResult, error) {
	r, err := b.prepareGuest(ctx, source, s, false)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	return b.Capabilities.Request(ctx, r)
}
func (b *Broker) DownloadFromGuest(ctx context.Context, source GuestSource, s GetSpec, sink io.Writer) (core.CapabilityResult, error) {
	if sink == nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	r, err := b.prepareGuest(ctx, source, ListSpec(s), true)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	return b.Capabilities.Request(context.WithValue(ctx, downloadSinkKey{}, sink), r)
}
func (b *Broker) prepareGuest(ctx context.Context, source GuestSource, s ListSpec, get bool) (core.CapabilityRequest, error) {
	if source.Environment == "" || !core.ValidEnvironmentInstanceID(source.Instance) || s.Environment != "" {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	s.Environment = source.Environment
	return b.prepareBound(ctx, s, get, source.Instance)
}
