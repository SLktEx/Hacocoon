# ADR 0110: Admit private egress while isolating Physical Host loopback

[日本語](0110-private-egress-and-loopback-isolation.ja.md) | English

Status: accepted. Refs #712.

## Decision

The Standard HTTP/HTTPS proxy admits global-unicast DNS answers, including
private IPv4 and IPv6 ULA, only after the existing exact hostname, Environment,
protocol and port grant. The public-only address requirement in
[ADR 0007](0007-controller-owned-standard-egress.md) is superseded. Policy,
Approval, source ownership and CONNECT SNI checks retain their owners.

The proxy runs in the Physical Host controller's network namespace. Reject
loopback answers even after hostname approval: they would reach Physical Host
services rather than the Environment's localhost. Retain Environment-local
`NO_PROXY=localhost,127.0.0.1,::1` in runtime and SSH settings.

Use Go's `netip.Addr.IsGlobalUnicast` after unmapping IPv4-mapped IPv6. Reject
malformed/zoned answers and reserved IPv4 `0.0.0.0/8` / `240.0.0.0/4` as well.
Non-unicast, link-local and loopback destinations are not ordinary unicast
upstreams; link-local access also requires interface scope and can expose local
infrastructure. Private/shared address space alone is not a refusal rationale.
The Host's route and eventual dial determine reachability; neither public nor
private addressing establishes application trust.

Reject the entire answer set if any answer is disallowed. Pin and deduplicate
all allowed answers before the first dial; fallback uses only that set, without
re-resolving the hostname. A later connection needs a fresh grant and lookup.

HTTP/CONNECT failure boundaries log one ERROR with selected target fields and
a fixed reason separating lookup, empty answers, loopback/other address refusal,
dial and request failures. Cancellation and deadline classifications take
precedence. Never log raw backend errors, request URLs, headers or bodies.

## Rejected alternatives and validation

A public-only filter overrides legitimate hostname authorization for private
networks. Allowing every address exposes Physical Host loopback and link-local
services. Filtering individual unsafe answers silently hides the rejected
addresses and permits partial success for an unsafe response. Dialing a hostname
again discards the inspected address set. Broadening NO_PROXY would bypass the
ordinary proxy path and is unnecessary.

The [egress design](../design/egress-authorization.md) owns the refusal and
observation contract. Component regressions cover RFC1918/ULA and mixed allowed
sets, mapped addresses, unsafe answers in either order, exact pinned dialing,
Policy refusal, HTTP/CONNECT diagnostics and runtime/SSH localhost settings.
These tests do not establish installed private-network or guest localhost
acceptance; real routing, VPN and provider behavior remain separate.
