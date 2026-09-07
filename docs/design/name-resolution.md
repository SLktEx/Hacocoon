# Environment name resolution

[日本語](name-resolution.ja.md) | English

Status: **partial**. The policy-bound resolver, controller HTTP route and UDP/TCP relay components exist. Installed Standard mode configures the guest service automatically on creation and resume. Real Windows/VPN acceptance of this slice remains pending.

## Intended ordinary use

Existing Environment creation and resume configure name resolution automatically, without a nameserver argument or a new command to remember. Ordinary applications will use getaddrinfo/getent. The owning decision is [ADR 0021](../adr/0021-policy-bound-name-resolution.md).

Lookup requires a `network.resolve` / `lookup` Policy decision for the Environment and canonical hostname. It does not grant HTTP, HTTPS or private-network connectivity. Existing allow/deny/approval evaluation and audit remain authoritative; this component adds no allow rules or policy UI. A denied lookup must not reach the platform resolver.

## Resolver ownership

Standard uses the Physical Host platform resolver. On supported Windows/WSL installations, Windows DNS tunneling is the intended upstream path; automatic WSL resolv.conf generation must remain enabled. No public resolver fallback is configured. See [Microsoft's DNS tunneling guidance](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling).

The untrusted guest stub uses loopback port 53 and the existing fixed proxy endpoint. It receives no Host credential or management handle. Source ownership is resolved from the guarded peer identity. The relay supports only bounded IN A/AAAA requests and does not cache answers. The trusted Host's separate infrastructure DNS path requires its own acceptance.

## Change propagation and acceptance still required

Windows DNS changes, VPN connect/disconnect and WSL restart must be checked against the Windows resolver, Physical Host, trusted Host and Environment using the same public and available VPN names. Platform and application caching must be distinguished from the uncached relay. Current component tests do not establish when any particular VPN change becomes visible.

Maintained component tests cover policy/audit refusal, source-header spoofing, private-address results without connection grants, malformed messages, UDP/TCP and cancellation. Automatic guest provisioning is implemented; actual getaddrinfo acceptance is pending in the new Windows GHA fixture. VPN/NRPT and OS restart acceptance are not run; they must be reported as SKIP if no suitable host/VPN fixture is available. Connection allow/deny must be tested separately. Do not count proxy-only name resolution as guest acceptance.

Installer acceptance at `72096d8` failed during automatic DNS service setup on
both Ubuntu and Windows, before the getaddrinfo fixture. The failure is not
counted as SKIP or success. A local isolated unit-start probe succeeded on an
older installed substrate and was cleaned up; its success does not validate
the current installed create path. Failure phase/numeric service status are
allowlisted; raw guest logs and script contents are not forwarded.
