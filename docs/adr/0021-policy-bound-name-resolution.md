# Policy-bound Environment name resolution

Status: accepted design; partial implementation.

## Context

Normal Environment applications need address lookup through the Physical Host resolver. On Windows/WSL this is intended to retain Windows DNS tunneling and VPN name policy. Proxy-side resolution alone does not satisfy ordinary guest getaddrinfo. Opening direct DNS would introduce an unaudited egress path.

## Decision

Core uses the existing Capability service with `network.resolve`, action `lookup`, a canonical hostname resource and the source Environment identity. Policy and durable audit precede resolution. Lookup grants no connection authority. Private addresses may be returned without allowing connections to them.

A replaceable Standard adapter resolves A/AAAA addresses through the Physical Host's platform resolver. It has no guest-selected upstream and no fixed public DNS fallback. A guest loopback UDP/TCP stub relays bounded DNS messages to the existing guarded HTTP endpoint at `169.254.254.1:18080/_haco/dns-query`. The controller identifies the source using persisted provider ownership, never a guest header. Existing direct-network restrictions remain.

The planned installed integration configures the stub automatically during canonical Environment creation. It must expose no management socket, Windows executable, drive or Host credential. The trusted Host retains its infrastructure network; acceptance must separately verify its actual resolver path.

## Limits and failures

Only one IN A/AAAA question is supported, at most 4096 wire bytes and 32 addresses. Work is bounded to 32 concurrent requests and a five-second Host lookup deadline. Unsupported requests cannot invoke the resolver. Policy rejection returns REFUSED; unavailable approval, audit or resolver fails closed. Relay answers have zero TTL and no relay cache. Platform/application caches still exist.

No default allow rule is added. DNS-only permission is explicit and distinct from egress permission. Denial and headless approval cannot contact the upstream resolver. Guest UDP truncates large answers and permits bounded TCP retry. Cancellation closes listeners and active TCP connections.

## Rejected alternatives

- Direct guest access to arbitrary DNS servers would bypass policy and source enforcement.
- Copying nameserver addresses into every Environment would become stale and may bypass Windows VPN policy.
- Passing Windows execution or Host control authority into the guest is unnecessary.
- Automatically turning a DNS answer into a connection grant would conflate discovery and access.

## Current scope

The broker, Standard provider, HTTP handler and guest stub are implemented and tested as components. The controller routes the handler through its existing Standard listener, but the guest stub is not yet automatically installed. Automatic provisioning, getaddrinfo acceptance, Windows/VPN propagation and connectivity-policy acceptance remain incomplete. See [the design contract](../design/name-resolution.md).
