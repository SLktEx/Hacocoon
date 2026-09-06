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

The canonical Environment provider uses one owned bridge per Environment, with NAT disabled, DHCP enabled and DNS disabled. Verified Host inet rules and per-Environment source guards enforce proxy-only access; the trusted `haco-host` NAT bridge is a separate infrastructure path. Proxy environment variables do not weaken the lower-layer boundary. See [managed Environment networking](design/managed-sandbox-network.md) for the authoritative topology and retained legacy paths.

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

## Operational path

The installed unit runs `haco-controller --standard-egress`. This serves the existing composition's Standard proxy, Policy, audit and persisted source resolver on the fixed endpoint after the Incus adapter verifies its guards. A bare controller is available for isolated control-transport use; the installer always enables the Standard service. New `haco` needs no egress-serving command, and the retained `hacoq egress serve` is legacy functionality.

Controller and proxy shutdown are coupled. Every accepted proxy connection, including a hijacked CONNECT tunnel, closes on shutdown. Requests are canceled during ClientHello, upstream writes and established forwarding. Headers are limited to 16 KiB, header reads to 10 seconds and retained connections to 256. HTTP transport failures use a fixed structured log message without raw panic output.

The daemon never consumes stdin as ambient approval. Missing Policy denies traffic. An exact allow uses the existing protected Physical Host policy file and audit contract; `require-approval` without an interactive provider is refused. The service adds neither an approval UI nor an automatic allow policy. Installed Environment traffic acceptance passed with explicit administrator Policy configuration; an ordinary policy-management UI remains follow-up work. See [ADR 0007](adr/0007-controller-owned-standard-egress.md).

Git push remains a separate privileged operation through the Git boundary and must not be enabled by handing reusable Host Git credentials to an Environment.

## Acceptance boundary

The Windows workflow adds a separate installed-controller packet check after the exact BAT journey succeeds. An ordinary Physical Host API client creates one read-only Workspace/Environment, executes a static HTTPS probe from that Workspace, and deletes through the same controller. It starts no second controller and uses no legacy CLI or product environment override. The documented administrator `policy.json` operation grants only that Environment's `github.com` HTTPS port 443; an existing Policy is never overwritten, and cleanup removes only the unchanged acceptance Policy. This is explicit policy configuration, not installer or network repair.

The probe requires certificate-verified HTTPS through the installed proxy, proxy 403 for an unapproved hostname, and refusal of a direct TCP connection to a public endpoint first proved reachable from the Physical Host. It also checks that management socket paths are absent. Guest route startup is only observed; no packages, NAT exceptions, firewall changes, service overrides or mount repairs are injected. This is controller/provider packet acceptance, not a claim that the planned product Environment CLI or ordinary policy UI is implemented. Commit-bound results belong in implementation status.

Repository tests cover allow/deny/require-approval integration, direct-IP rejection, shared-IP/alternate-hostname resistance, mixed/private DNS answers, SNI mismatch, legacy network migration, unmanaged DNS/ACL drift and trusted source-IP mapping. Real supported-Incus bridge/nftables/dnsmasq behavior remains a host acceptance concern and must not be inferred solely from unit/static tests.
