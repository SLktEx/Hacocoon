# Domain-aware egress authorization

Status: **authorization/proxy engine implemented in the repository; Incus routing/profile integration remains pending in #177.**

Hacocoon must authorize outbound access by the hostname a sandbox asks to reach. Resolving a name once and allowing the resulting IP addresses is not equivalent: shared CDNs, DNS changes and direct-IP access would make that authority transferable to unrelated destinations.

## Boundary

The egress request/authorization contract belongs to Core. The project-maintained HTTP/HTTPS enforcement proxy is a Standard implementation. Incus-specific routing/ACL/profile plumbing stays in the Incus adapter.

```text
sandbox
  -> default-deny network substrate
  -> Standard egress proxy
  -> network.egress/connect Capability
  -> Policy: allow / require-approval / deny
  -> Host DNS resolution + pinned public address
  -> upstream
```

The proxy is not an approval token issuer and does not cache an approval as an IP allowlist. Every grant is scoped to one Environment, canonical hostname, protocol and port, and to one connection attempt.

## Implemented engine

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

## Policy example

The first engine uses the existing exact-resource policy model. A permanent allow for one HTTPS hostname can be written as:

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

## Deliberate non-goals and remaining integration

This repository slice does **not** yet make sandbox traffic reach the proxy. #177 remains open for the Incus/Standard integration that must:

- make the proxy the only ordinary outbound transport reachable from a sandbox;
- keep direct arbitrary IP traffic rejected below the application layer;
- prevent the Incus bridge recursive DNS service from becoming an exfiltration path;
- derive the Environment identity from trusted network/runtime state rather than a caller-provided label;
- configure standard proxy discovery for sandbox workloads without exposing Host credentials;
- provide deterministic restart/recovery and real supported-Incus acceptance.

Git push remains a separate privileged operation through the Git plugin and must not be made to work by giving reusable Host Git credentials to the sandbox.
