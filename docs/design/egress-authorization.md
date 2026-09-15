# Domain-aware egress authorization

Status: **implemented; installed Windows proxy allow/deny and direct-egress refusal acceptance passed.**

Hacocoon authorizes outbound access by the hostname a sandbox asks to reach. Resolving a name once and allowing the resulting IP addresses is not equivalent: shared CDNs, DNS changes and direct-IP access would make that authority transferable to unrelated destinations.

## Boundary

The egress request/authorization contract belongs to Core. The project-maintained HTTP/HTTPS enforcement proxy is a Standard implementation. Incus-specific bridge, ACL, profile and source-identity plumbing stays in the Incus adapter.

```text
sandbox
  -> dedicated Environment bridge + Host traffic guards
  -> only 169.254.254.1:18080 is allowed for application egress
  -> Standard egress proxy
  -> trusted source-IP -> Environment resolution
  -> network.egress/connect Capability
  -> Policy: allow / require-approval / deny
  -> Host DNS resolution + pinned public address
  -> upstream
```

The proxy is not an approval-token issuer and does not cache approval as an IP allowlist. Every grant is scoped to one Environment, canonical hostname, protocol and port, and to one connection attempt.

The serving proxy tracks accepted clients and CONNECT upstream connections in
one shutdown owner. Shutdown closes both sides synchronously; a dial that
finishes after shutdown is rejected before its first upstream write. Context
cancellation alone is insufficient because its callbacks run asynchronously.
Repository regression covers ClientHello wait, prefix write, established tunnels
and a late upstream registration. This changes no policy grants or CLI steps.

## Implemented authorization engine

- `internal/core` defines provider-neutral `EgressRequest` / `EgressGrant` values.
- `internal/egress` canonicalizes DNS hostnames and routes `network.egress/connect` through the existing Policy / Approval / Capability / audit boundary.
- IP literals are rejected before policy evaluation.
- `modules/standard/egressproxy` implements explicit HTTP and HTTPS proxy enforcement.
- HTTP absolute-target and `Host` authorities must identify the same hostname and port.
- DNS is resolved on the trusted Host only after hostname authorization.
- The resolved address set is pinned for that connection; dialing does not resolve the hostname again.
- Private, loopback, link-local, CGNAT, benchmark, documentation, multicast and other unsafe addresses are rejected. A mixed public/private answer fails as a whole rather than depending on resolver order.
- HTTPS `CONNECT` is not trusted by itself. Before any TLS bytes are forwarded upstream, the proxy parses a bounded TLS ClientHello and requires SNI to canonicalize to the same hostname as the authorized CONNECT target.
- Provider/audit failures remain fail-closed through the existing Capability service.

## Implemented Incus enforcement

The canonical Environment provider uses one owned bridge per Environment, with NAT disabled, DHCP enabled and DNS disabled. Verified Host inet rules and per-Environment source guards enforce proxy-only access; the trusted `haco-host` NAT bridge is a separate infrastructure path. Proxy environment variables do not weaken the lower-layer boundary. See [managed Environment networking](managed-sandbox-network.md) for the authoritative topology and retained legacy paths.

The proxy resolves its peer through trusted Incus runtime state and the controller's persisted Environment store, rather than accepting an Environment name from the guest. It listens only on the fixed Physical Host endpoint `169.254.254.1:18080`. Missing, ambiguous or unmanaged source identities fail closed. Restart does not retain a connection grant or turn a hostname grant into an IP allowlist.

Persisted runtime references include their provider route. Source binding uses the Environment router's reference decoder and requires both the configured source provider and its native runtime reference to match. An identical native reference under another provider grants no authority.

## Policy example

The implementation uses the existing exact-resource policy model. A permanent allow for one HTTPS hostname can be written as:

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "network.egress",
      "action": "connect",
      "resource": "api.example.com",
      "environment": "env-a",
      "attributes": {"protocol": "https", "port": "443"},
      "decision": "allow",
      "reason": "approved development API"
    }
  ]
}
```

Use `require-approval` instead of `allow` when the existing approval provider must approve each connection request. The Environment, hostname, protocol and port stay in the audited authority scope.

## Product package-repository baseline

The overall Environment policy remains default-deny, but Hacocoon has one narrow
product-owned egress baseline for the Ubuntu package repositories used by the
official Base contract:

- `archive.ubuntu.com` on HTTP 80 and HTTPS 443;
- `security.ubuntu.com` on HTTP 80 and HTTPS 443;
- `ports.ubuntu.com` on HTTP 80 and HTTPS 443.

These are exact `network.egress/connect` destinations, not permission granted to
the `apt` process. Guest `/etc/apt/sources*` files are never read to extend this
list, so adding a PPA, third-party repository or arbitrary mirror does not grant
network authority. The fixed baseline also remains useful for custom Bases built
from an official Ubuntu Base that retain the same standard repositories.

Matching administrator or saved Policy rules are evaluated first. The existing
`deny > require-approval > allow` precedence therefore lets an administrator
restrict or approval-gate any baseline destination. Only when no explicit rule
matches is the package baseline considered, followed by the configured Policy
default. The baseline is product code, not revision-bound operator configuration,
so `haco config` and `policy.json` do not serialize it. See
[ADR 0063](../adr/0063-default-package-repository-egress.md).

## Operational path

The installed unit runs `haco-controller --standard-egress`. This serves the existing composition's Standard proxy, Policy, audit and persisted source resolver on the fixed endpoint after the Incus adapter verifies its guards. A bare controller is available for isolated control-transport use; the installer always enables the Standard service. New `haco` needs no egress-serving command, and the retained `hacoq egress serve` is legacy functionality.

Controller and proxy shutdown are coupled. Every accepted proxy connection, including a hijacked CONNECT tunnel, closes on shutdown. Requests are canceled during ClientHello, upstream writes and established forwarding. Headers are limited to 16 KiB, header reads to 10 seconds and retained connections to 256. HTTP transport failures use a fixed structured log message without raw panic output.

The daemon never consumes ambient stdin. Missing Policy denies traffic except for
the fixed package-repository baseline above. Controller require-approval waits in
a bounded Standard queue for `haco approve` on the trusted Host. Persistence and
execution still pass through the existing Policy, audit and identity checks. Use
`haco config` for ordinary operator Policy editing; add an explicit matching
`deny` or `require-approval` to restrict a package baseline destination. See
[pending review](pending-approval-review.md) and
[ADR 0028](../adr/0028-pending-approval-sessions.md). Installed acceptance of the
review path remains separate.

Git push remains a separate privileged operation through the Git boundary and must not be enabled by handing reusable Host Git credentials to an Environment.

## Acceptance boundary

The Windows workflow adds installed-controller checks after the exact BAT journey
succeeds. Before creating any acceptance Policy, an ordinary Physical Host API
client creates a disposable Environment and runs quiet `apt-get update` plus a
standard package reinstall. It then writes a temporary third-party APT source for
`example.com` and proves that the Standard proxy still returns 403 for that host.
This demonstrates both that the fixed package repositories work by default and
that guest source-list edits cannot widen the baseline.

A separate packet check then creates one read-only Workspace/Environment, executes
a static HTTPS probe from that Workspace, and deletes through the same controller.
It starts no second controller and uses no legacy CLI or product environment
override. The documented administrator `policy.json` operation grants only that
Environment's `github.com` HTTPS port 443; an existing Policy is never overwritten,
and cleanup removes only the unchanged acceptance Policy. This is explicit policy
configuration, not installer or network repair.

The probe requires certificate-verified HTTPS through the installed proxy, proxy 403 for an unapproved hostname, and refusal of a direct TCP connection to a public endpoint first proved reachable from the Physical Host. It also checks that management socket paths are absent. Guest route startup is only observed; no NAT exceptions, firewall changes, service overrides or mount repairs are injected. This is controller/provider packet acceptance, not a claim that the planned product Environment CLI or ordinary policy UI is implemented. Commit-bound results belong in implementation status.

Repository tests cover allow/deny/require-approval integration, direct-IP rejection, shared-IP/alternate-hostname resistance, mixed/private DNS answers, SNI mismatch, legacy network migration, unmanaged DNS/ACL drift and trusted source-IP mapping. Baseline regression also pins the exact package host/protocol/port set and proves explicit restrictions outrank it. Real supported-Incus bridge/nftables/dnsmasq behavior remains a host acceptance concern and must not be inferred solely from unit/static tests.

## Source observation ownership

Implemented: the Incus adapter returns native runtime references only. The unused
direct Environment-name derivation helper has been removed; persisted source
binding remains the sole production Environment identity resolver. Failed,
canceled or truncated Incus output cannot establish a source, even if it includes
one plausible name. Normalization, Policy/Approval and concrete Standard dialing
retain their existing owners. HTTP/HTTPS support and public commands are unchanged.

## Transport implementation structure

Implemented: Standard proxy composition/routing, HTTP forwarding, CONNECT,
authority parsing, TLS ClientHello parsing and pinned dialing have separate
responsibilities. HTTP/CONNECT share exact grant consumption and authorized DNS
resolution; Core Policy/Approval is not copied into the enforcer. Mismatched grant
fields fail before DNS. Cancellation is checked around resolution and dialing;
late connections close without upstream writes. Current HTTP status bodies,
hostname normalization, SNI checks and the one-attempt grant scope remain.
See [the existing ADR](../adr/0007-controller-owned-standard-egress.md#shared-transport-admission).
