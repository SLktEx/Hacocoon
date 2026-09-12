# Managed Environment network

Status: **implemented for the canonical Environment provider; installed Windows proxy allow/deny and direct-egress refusal acceptance passed.**

The current Incus SandboxProvider creates one dedicated managed bridge per Environment on Linux and WSL. The older shared `haco-sandbox0` / ACL / profile helpers remain in legacy Runtime and Seed paths; they are not the current Environment topology and must not become its fallback.

## Current topology and ownership

An Environment starts without inherited profiles. Its explicit NIC attaches to a deterministic `hbr*` bridge in the Incus default resource project. The production command adapter marks creation with `user.hacocoon.owner=environment-network-v1` and verifies that marker before attachment or deletion. A matching name does not establish ownership.

The bridge uses Incus-selected IPv4 addressing, DHCP and routing, with `ipv4.nat=false`, `ipv4.firewall=true`, `ipv6.address=none` and `raw.dnsmasq=port=0`. DNS service is disabled. Incus' IPv4 firewall remains enabled for DHCP/checksum plumbing; the earlier Hacocoon inet hooks enforce the traffic boundary. The NIC has a fixed managed MAC and port isolation.

The shared proxy endpoint is `169.254.254.1:18080`, a Physical Host loopback address, not each bridge gateway. The adapter supplies upper/lowercase HTTP(S) proxy settings and local-only NO_PROXY values. Those convenience variables grant no authority.

## Traffic boundary

The adapter verifies its shared nftables input/forward rules. Environment-initiated Host traffic is limited to DHCP and the fixed proxy endpoint; established replies to Host-initiated traffic are separate. Direct forwarding to the outside or another Environment is dropped. A per-Environment prerouting guard pins the managed MAC and IPv4 subnet, with only the pre-address DHCP tuple exempted from the subnet check.

Each Environment has its own bridge; the shared-L2 assumptions of the older network do not apply. The proxy maps a connection source through trusted Incus runtime state and the controller's persisted Environment identity. Hostname authorization, public-address pinning and HTTPS SNI verification belong to the replaceable Standard proxy and Core Capability contracts, as described in [egress authorization](../EGRESS_AUTHORIZATION.md).

The persistent trusted `haco-host` uses a different, owned NAT bridge for infrastructure connectivity. Its allowed DNS/HTTPS traffic must never be used as evidence that an Environment can bypass its proxy. See [trusted-host networking](trusted-host.md#dedicated-trusted-host-network).

## Implementation limits

Several helpers and constants still contain `Routed` / `routed` migration names even though the canonical data plane is now bridge based. Do not infer routed-NIC behavior from those names. The retained shared bridge helpers and their tests are legacy coverage, not permission to attach current Environments to that NAT path.

The installed unit now enables the Standard proxy inside the existing Physical Host controller. The adapter verifies its shared guards before the fixed listener is bound; preparation/bind failure prevents controller service startup, and either service exiting stops its peer. Shutdown closes hijacked CONNECT sockets as well as ordinary HTTP sockets. Headless require-approval fails closed. See [ADR 0007](../adr/0007-controller-owned-standard-egress.md). This lifecycle and packaged Environment allow/deny are accepted on Windows with explicit administrator Policy configuration; an ordinary policy-management UI remains follow-up work.

## Acceptance

Repository tests cover ownership, network/guard configuration, lifecycle and source identity. Real-Incus gates exercise the provider separately from installed Windows acceptance. The exact Windows installer gate proves trusted-host infrastructure connectivity and retention. Its separate installed-controller check passed Environment proxy allow/deny and direct TCP refusal. Firewall reload/startup ordering and live Docker coexistence remain separate acceptance concerns. See [implementation status](../IMPLEMENTATION_STATUS.md).

## Policy-bound name resolution

The Standard listener also routes bounded DNS queries through a separate lookup Capability. This does not enable direct DNS or grant connections to returned addresses. Automatic guest stub provisioning is implemented in installed Standard mode; see [name resolution](name-resolution.md) and [ADR 0021](../adr/0021-policy-bound-name-resolution.md).

Stopped owned Environments restore absent volatile source guards before start; existing drift and missing running-guest guards fail closed. See [ADR 0022](../adr/0022-resume-volatile-source-guards.md).


## Installed Windows source-guard observation

Implemented in the Windows SSH acceptance fixture: after ordinary Env creation and SSH preparation, a read-only observer pins the instance generation and checks its running state, isolated NIC, owned non-NAT bridge, and the native nftables source table. The table must contain exactly the ordered MAC rejection, narrow DHCP bootstrap exception, and IPv4 subnet rejection at prerouting priority -300. Unexpected rules, chains, identities or incomplete queries fail the check; the observer never repairs state. Generation and NIC identity are read again after observation.

This supplements installed HTTPS/proxy/direct-TCP acceptance. It checks actual kernel configuration, not delivery of spoofed packets, another Env's deletion, or a full reboot/recreate sequence. Those scopes must not be inferred from this observer. Windows integration runs in the maintained SSH gate; its native outcome must be recorded separately from the observer regressions.

Scoped local acceptance passed on a dedicated Incus/WSL installation: after canonical start of a stopped recovered Env, this observer verified its pinned generation and native guard rules. The first startup attempt failed before controller socket readiness; an earlier standalone observation also failed. These failures are not successes. The full packaged Windows SSH gate and spoofed-packet behavior remain separate acceptance.

## Policy-bound development relay

The Standard endpoint also supports [explicit TCP/UDP development
connections](network-connections.md). Ordinary clients select a guest loopback
listener. Existing HTTP/SNI behavior, bridge source guards and default packet
denial remain unchanged. Private management and Incus socket access are not
forwarded through the guest endpoint.
