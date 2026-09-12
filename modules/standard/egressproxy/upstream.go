package egressproxy

import (
	"net/http"
	"net/netip"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// prepareUpstream is shared transport admission for HTTP and CONNECT. The
// authorizer owns Policy/Approval; this Standard layer verifies the returned
// scope and pins public DNS answers. It does not dial, cache or widen a grant.
// CONNECT must still verify ClientHello SNI before using the pinned addresses.
func (p *Proxy) prepareUpstream(w http.ResponseWriter, r *http.Request, target core.EgressRequest) ([]netip.Addr, bool) {
	grant, err := p.authorizer.Authorize(r.Context(), target)
	if err != nil || grant.Environment != target.Environment || grant.Host != target.Host || grant.Port != target.Port || grant.Protocol != target.Protocol {
		http.Error(w, "egress denied", http.StatusForbidden)
		return nil, false
	}
	addresses, err := p.resolvePinned(r.Context(), target.Host)
	if err != nil {
		http.Error(w, "upstream resolution denied", http.StatusBadGateway)
		return nil, false
	}
	return addresses, true
}
