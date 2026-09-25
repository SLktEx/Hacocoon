package egressproxy

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

// prepareUpstream is shared transport admission for HTTP and CONNECT. The
// authorizer owns Policy/Approval; this Standard layer verifies the returned
// scope and pins allowed DNS answers. It does not dial, cache or widen a grant.
// CONNECT must still verify ClientHello SNI before using the pinned addresses.
func (p *Proxy) prepareUpstream(w http.ResponseWriter, r *http.Request, target core.EgressRequest) ([]netip.Addr, bool) {
	grant, err := p.authorizer.Authorize(r.Context(), target)
	if err != nil || grant.Environment != target.Environment || grant.Host != target.Host || grant.Port != target.Port || grant.Protocol != target.Protocol {
		http.Error(w, "egress denied", http.StatusForbidden)
		return nil, false
	}
	addresses, err := p.resolvePinned(r.Context(), target.Host)
	if err != nil {
		logUpstreamFailure(r.Context(), target, err)
		http.Error(w, "upstream resolution denied", http.StatusBadGateway)
		return nil, false
	}
	return addresses, true
}

// Keep arbitrary resolver/dialer text out of both responses and logs, while
// preserving error identity for callers. Only fixed reasons cross the log boundary.
type upstreamFailure struct {
	reason string
	err    error
}

func (e *upstreamFailure) Error() string { return e.reason }
func (e *upstreamFailure) Unwrap() error { return e.err }

func logUpstreamFailure(ctx context.Context, target core.EgressRequest, err error) {
	reason := "upstream_request_failed"
	var failure *upstreamFailure
	switch {
	case errors.Is(err, context.Canceled):
		reason = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "timeout"
	case errors.As(err, &failure):
		reason = failure.reason
	}
	logging.FromContext(ctx).ErrorContext(ctx, "proxy upstream failed",
		"component", "proxy", "operation", "egress_connect",
		"environment_id", target.Environment, "target_host", target.Host,
		"target_port", target.Port, "protocol", target.Protocol, "reason", reason)
}
