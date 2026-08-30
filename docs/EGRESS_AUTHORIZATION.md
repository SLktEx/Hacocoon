# Domain-aware egress authorization

Status: **repository implementation complete; real supported-Incus acceptance remains host-dependent.**

Hacocoon authorizes outbound access by the hostname a sandbox asks to reach. Resolving a name once and allowing the resulting IP addresses is not equivalent: shared CDNs, DNS changes and direct-IP access would make that authority transferable to unrelated destinations.

## Boundary

The egress request/authorization contract belongs to Core. The project-maintained HTTP/HTTPS enforcement proxy is a Standard implementation. Incus-specific bridge, ACL, profile and source-runtime lookup stay in the Incus adapter; binding that provider-local runtime identity to persisted Hacocoon Environment identity stays in the Environment layer.

```text
sandbox
  -> Incus NIC default deny
  -> only 10.x bridge-gateway:18080 is transport-allowed
  -> Standard egress proxy
  -> trusted source-IP -> provider runtime ref
  -> persisted Environment binding
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

- The managed bridge keeps DHCP but sets `raw.dnsmasq=port=0`, disabling the bridge DNS service so guest DNS cannot become a separate exfiltration path.
- The Hacocoon NIC still defaults unmatched ingress and egress to `reject`.
- The managed ACL contains exactly one ordinary outbound allow rule: TCP to the Hacocoon bridge gateway on the Standard proxy port (`18080`). An empty legacy v0.13 ACL is migrated to that rule; unmanaged ACL rules fail closed.
- Existing managed bridges with an empty `raw.dnsmasq` value are migrated to `port=0`; any other operator/custom value is rejected rather than overwritten.
- The managed `haco-sandbox` profile injects uppercase and lowercase `HTTP_PROXY` / `HTTPS_PROXY` values that point to the bridge proxy and local-only `NO_PROXY` values. Direct traffic still cannot bypass the proxy because the NIC ACL remains authoritative below those convenience environment variables.
- The proxy resolves the connection source IP through trusted Incus runtime state to exactly one provider-local runtime ref, then requires that exact provider/runtime ref to belong to exactly one persisted managed Hacocoon Environment before assigning Environment-scoped policy authority. Missing, orphaned, ambiguous or unpersisted matches are denied.
- `haco egress serve` verifies the managed network and listens only on the Hacocoon bridge gateway. It runs in the trusted Host foreground so the existing synchronous stdio `require-approval` provider remains usable.
- The broker is intentionally stateless across restart. If it is absent or restarting, the only ACL-allowed transport has no listener and outbound access fails closed.

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

Run the Standard broker on the trusted Host:

```text
haco egress serve
```

The listen address is not caller-selectable. Hacocoon derives it from the managed Incus bridge and verifies the bridge, ACL and profile before accepting traffic.

Git push remains a separate privileged operation through the Git plugin and must not be made to work by giving reusable Host Git credentials to the sandbox.

## Acceptance boundary

Repository tests cover allow/deny/require-approval integration, direct-IP rejection, shared-IP/alternate-hostname resistance, mixed/private DNS answers, SNI mismatch, legacy network migration, unmanaged DNS/ACL drift, trusted source-IP lookup and persisted Environment binding including orphan rejection. Real supported-Incus bridge/nftables/dnsmasq behavior remains a host acceptance concern and must not be inferred solely from unit/static tests.
