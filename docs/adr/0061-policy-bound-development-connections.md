# ADR 0061: Relay explicitly authorized development connections

Status: accepted
Date: 2026-09-12

## Context

The current Environment owns an isolated Incus bridge and can reach one guarded
Standard endpoint. The HTTP/HTTPS proxy, source ownership checks, guest companion,
name resolver and Policy/Approval/Audit boundary already exist. Ordinary TCP and
UDP tools also need access to selected external, Host and Environment services.

## Decision

Extend that endpoint with a bounded Standard TCP/UDP relay and expose explicit
loopback listeners through the existing guest product companion. Keep connection
contracts separate from the HTTP hostname/SNI contract. The Standard component
owns sockets, peer pinning, framed UDP associations and cancellation; the Incus
adapter owns destination identity and source mapping. Core retains Capability
and lifetime contracts without owning a new routing fabric.

A local listener is convenience, not authority. Each association obtains a
fresh source-generation-bound decision before upstream dialing and after
separately authorized DNS. Host services require an explicit trusted registration.
Environment destinations use provider-verified, pinned kernel namespaces so
same-name/IP reuse cannot redirect TCP or UDP. External sockets bind the observed
outgoing device so new managed routes cannot capture old UDP associations. No controller or Incus API is
exposed through the guest route. Established connections close on expiry,
revocation or invalidated authority. Nothing is automatically replayed at restart.

## Rejected alternatives

- Blanket bridge/NAT exceptions would expand the existing packet boundary and
  require new per-packet revocation and source/destination-reuse machinery.
- HTTP CONNECT without the existing HTTPS SNI check would silently weaken the
  established HTTP/HTTPS contract.
- DNS allowlists alone cannot authorize connections or identify a recreated Env.
- SOCKS/proxy variables alone exclude ordinary tools without that support.
- A reusable guest credential or management socket would cross the trust boundary.
- Shared writable Workspace or OCI state is unrelated to connection authority.

## Consequences

Clients select a local endpoint explicitly; this is not transparent unrestricted
routing. TCP streams and UDP associations add a relay hop and have explicit
lifetimes. The existing Incus isolation remains the lower-layer boundary.
See the [owning contract](../design/network-connections.md) for detailed behavior
and separate repository, real-provider and desktop acceptance requirements.
