# ADR 0005: Own trusted-host networking independently of storage

Status: accepted  
Date: 2026-09-06

## Context

The WSL installer inferred Incus network readiness from the presence of storage,
then used `incus admin init --minimal`. This created an unused directory pool and
made trusted `haco-host` inherit the mutable default profile. Windows acceptance
also found HTTPS timeouts when Docker's FORWARD policy was DROP; later boots with
ACCEPT succeeded without a product change. A controller ping alone was therefore
not proof that trusted tooling could reach external services.

## Decision

The Incus adapter creates and verifies the dedicated managed bridge `haco-host0`
in the default resource project. Creation records
`user.hacocoon.owner=trusted-host-network-v1` with IPv4 DHCP, managed DNS, NAT,
routing and Incus firewall enabled; IPv6 is disabled for this initial contract.
Existing objects require the exact owner, bridge type, private IPv4 subnet and
known configuration. Unknown consumers, routing/DNS overrides and external
interfaces fail closed. Failed readback does not delete a persistent resource.

Fresh trusted instances use an explicit local root disk and NIC with no profiles.
The installer checks daemon readiness and leaves pool/network creation to the
adapter. It does not initialize Incus's default storage/profile/network.

Current installed data is retained: an exactly owned trusted instance with its
known inherited `incusbr0` NIC and default profile may transition once. Validate
the local root and every inherited device, stop gracefully, make the owned NIC
explicit and remove the profile from that instance. Never change the shared
profile, delete its former network/pool, force-stop the instance, or reset its
root disk/UUID. Interrupted transition retains ownership and is resumable. This
limited current-install transition is not a general old-installer compatibility
path and must be removed when that transition is no longer needed.

Before bootstrap or a trusted-host entry, reconcile Docker's documented
`DOCKER-USER` extension point if present. Allow only the verified bridge/subnet's
outbound packets and established/related replies. Keep global FORWARD policy and
unrelated rules unchanged. A DROP policy without that extension point fails
explicitly. No rules are added for Environment bridges, and no second controller
or guest daemon is introduced. Existing matching rules are reused; concurrent
inserts can only duplicate the same narrow rule, and readback is required.

The installer verifies trusted-host DNS, a default IPv4 route and HTTPS before
reporting success. This is infrastructure connectivity, separate from the
Environment's allowed proxy path and forbidden direct egress.

## Rejected alternatives

- A global FORWARD ACCEPT policy, disabling firewall enforcement, or enabling
  Environment NAT would weaken the untrusted boundary.
- Reusing or rewriting the default profile/bridge would affect unrelated users
  and make trusted connectivity depend on mutable external configuration.
- Inferring network readiness from storage would recreate the original bug.
- A second installer network implementation would split ownership from the
  controller's provider adapter.

## Validation and limits

Unit regressions cover ownership/configuration refusal, ambiguous creation,
malformed inventory, scoped/idempotent forwarding, interrupted NIC transition
and refusal to alter unknown profiles. A separate Linux network-namespace test
uses actual packets under Docker-like FORWARD DROP: trusted outbound/replies
must pass, unsolicited inbound and another NIC must fail, and global DROP must
remain. It is not a Windows product-preparation fixture.

Packaged Windows acceptance must use the ordinary cached BAT, preserve current
data, and inspect the network it creates. Linux CI continues to verify the
provider foundation. Reconciliation occurs at bootstrap/entry, not continuously;
an external firewall reload or Docker startup during an already-open session
can still interrupt connectivity until another entry/reconciliation. Such live
changes and arbitrary third-party firewalls are not claimed as accepted.

See [trusted Host](../design/trusted-host.md) and
[Incus firewall coexistence](https://linuxcontainers.org/incus/docs/main/howto/network_bridge_firewalld/#prevent-connectivity-issues-with-incus-and-docker).
