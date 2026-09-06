# Managed Environment network

Status: **implemented for the canonical Environment provider; installed proxy operation and Windows Environment egress acceptance remain partial.**

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

The installed controller currently constructs the Standard proxy but does not start its listener. Completing that service lifecycle and ordinary policy operation is M1 work. No listener means outbound access fails closed; it does not mean allowed proxy communication is usable.

## Acceptance

Repository tests cover ownership, network/guard configuration, lifecycle and source identity. Real-Incus gates exercise the provider separately from installed Windows acceptance. The exact Windows installer gate currently proves trusted-host infrastructure connectivity and retention. Environment allow/deny traffic on the installed product, firewall reload/startup ordering and live Docker coexistence require their own recorded acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md).
