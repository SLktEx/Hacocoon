# Domain-aware egress authorization

Status: **authorization/enforcement components are implemented; installed proxy service operation and Windows Environment egress acceptance are partial.**

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

The installed Physical Host controller constructs the Standard proxy but does not yet serve its listener. Consequently the installed Environment path is not accepted for allowed outbound traffic. The new product `haco` has no egress-serving command. The retained migration binary's `hacoq egress serve` foreground command is legacy functionality, not the installer service or a second controller to add to `haco-host`.

Completing the installed service lifecycle must reuse the existing controller's Policy, persisted source resolver and Standard proxy, including fail-closed shutdown and `require-approval` behavior when no interactive approval provider is available. It must not grant NAT/direct access to make an absent broker appear functional.

Git push remains a separate privileged operation through the Git boundary and must not be enabled by handing reusable Host Git credentials to an Environment.

## Acceptance boundary

Repository tests cover allow/deny/require-approval integration, direct-IP rejection, shared-IP/alternate-hostname resistance, mixed/private DNS answers, SNI mismatch, legacy network migration, unmanaged DNS/ACL drift and trusted source-IP mapping. Real supported-Incus bridge/nftables/dnsmasq behavior remains a host acceptance concern and must not be inferred solely from unit/static tests.
