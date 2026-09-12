# Environment name resolution

[日本語](name-resolution.ja.md) | English

Status: **partial**. The policy-bound resolver, controller HTTP route and UDP/TCP relay components exist. Installed Standard mode configures the guest service automatically on creation and resume. Windows GHA accepted ordinary resolution and default denial at `c05528a`; VPN/change/restart acceptance remains pending.

## Intended ordinary use

Existing Environment creation and resume configure name resolution automatically, without a nameserver argument or a new command to remember. Ordinary applications will use getaddrinfo/getent. The owning decision is [ADR 0021](../adr/0021-policy-bound-name-resolution.md).

Lookup requires a `network.resolve` / `lookup` Policy decision for the Environment and canonical hostname. It does not grant HTTP, HTTPS or private-network connectivity. Existing allow/deny/approval evaluation and audit remain authoritative; this component adds no allow rules or policy UI. A denied lookup must not reach the platform resolver.

## Resolver ownership

Standard uses the Physical Host platform resolver. On supported Windows/WSL installations, Windows DNS tunneling is the intended upstream path; automatic WSL resolv.conf generation must remain enabled. No public resolver fallback is configured. See [Microsoft's DNS tunneling guidance](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling).

The untrusted guest stub uses loopback port 53 and the existing fixed proxy endpoint. It receives no Host credential or management handle. Source ownership is resolved from the guarded peer identity. The relay supports only bounded IN A/AAAA requests and does not cache answers. The trusted Host's separate infrastructure DNS path requires its own acceptance.

## Change propagation and acceptance still required

Windows DNS changes, VPN connect/disconnect and WSL restart must be checked against the Windows resolver, Physical Host, trusted Host and Environment using the same public and available VPN names. Platform and application caching must be distinguished from the uncached relay. Current component tests do not establish when any particular VPN change becomes visible.

Maintained component tests cover policy/audit refusal, source-header spoofing, private-address results without connection grants, malformed messages, UDP/TCP and cancellation. Automatic guest provisioning is implemented; actual getaddrinfo equality across Windows, WSL, trusted Host and Environment, plus default DNS denial, passed in Windows GHA run 34132173483 at `c05528a`. VPN/NRPT and OS restart acceptance are not run; they must be reported as SKIP if no suitable host/VPN fixture is available. Connection allow/deny must be tested separately. Do not count proxy-only name resolution as guest acceptance.

Installer acceptance at `72096d8` failed during automatic DNS service setup on
both Ubuntu and Windows, before the getaddrinfo fixture. The failure is not
counted as SKIP or success. A local isolated unit-start probe succeeded on an
older installed substrate and was cleaned up; its success does not validate
the current installed create path. Failure phase/numeric service status are
allowlisted; raw guest logs and script contents are not forwarded.

The installer diagnostic at `7eecbdf` narrowed the failure to `daemon-reload`.
Provisioning now waits up to 30 seconds for the guest systemd manager before
mutating service state. A failed reload is not retried. Shell regression tests
cover delayed readiness, timeout, and reload failure; real installer acceptance
of this correction passed Ubuntu and Incus at `c05528a`. The Windows DNS fixture and VS Code connection passed, but the workflow failed later in the project-setup test harness before setup execution.

## Repeated setup and service activation

Windows acceptance at 39b5ce4 reproduced an independent approval-setup failure
and observed the DNS service Result as start-limit-hit through the existing
pinned SSH connection. Each setup started its Env, which unconditionally restarted
the DNS service even when its verified companion and unit were unchanged.

Provisioning now compares the verified companion and canonical unit. Changes
still restart the service; an unchanged configuration uses systemd start, which
keeps an active service running and starts an inactive one. Manager readiness,
daemon reload, enablement, active-state checking and resolver configuration remain.
Systemd's start limit, peer identity, network Policy and ownership checks are not
relaxed. A failed start remains a failure. The repeated-setup regression simulates
the observed start limit; actual installed acceptance of this fix is pending.
