# Environment name resolution

[日本語](name-resolution.ja.md) | English

Status: **partial**. The policy-bound resolver, controller HTTP route and UDP/TCP relay components exist. Installed Standard mode configures the guest service automatically on creation and resume. Windows GHA accepted ordinary resolution and default denial at `c05528a`; VPN/change/restart acceptance remains pending.

## Ordinary use

Existing Environment creation and resume configure name resolution automatically, without a nameserver argument or a new command to remember. Ordinary applications use getaddrinfo/getent. The owning decision is [ADR 0021](../adr/0021-policy-bound-name-resolution.md).

Lookup requires a `network.resolve` / `lookup` Policy decision for the Environment and canonical hostname. It does not grant HTTP, HTTPS or private-network connectivity. Existing allow/deny/approval evaluation and audit remain authoritative; this component adds no allow rules or policy UI. A denied lookup must not reach the platform resolver.

## Resolver ownership

Standard uses the Physical Host platform resolver. On supported Windows/WSL installations, Windows DNS tunneling is the intended upstream path; automatic WSL resolv.conf generation must remain enabled. No public resolver fallback is configured. See [Microsoft's DNS tunneling guidance](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling).

The untrusted guest stub uses loopback port 53 and the existing fixed proxy endpoint. It receives no Host credential or management handle. Source ownership is resolved from the guarded peer identity. The relay supports only bounded IN A/AAAA requests and does not cache answers. The trusted Host's separate infrastructure DNS path requires its own acceptance.

## Change propagation and acceptance still required

Windows DNS changes, VPN connect/disconnect and WSL restart must be checked against the Windows resolver, Physical Host, trusted Host and Environment using the same public and available VPN names. Platform and application caching must be distinguished from the uncached relay. Current component tests do not establish when any particular VPN change becomes visible.

Maintained component tests cover policy/audit refusal, source-header spoofing, private-address results without connection grants, malformed messages, UDP/TCP and cancellation. Automatic guest provisioning is implemented; actual getaddrinfo equality across Windows, WSL, trusted Host and Environment, plus default DNS denial, passed in Windows GHA run 34132173483 at `c05528a`. VPN/NRPT and OS restart acceptance are not run; they must be reported as SKIP if no suitable host/VPN fixture is available. Connection allow/deny must be tested separately. Do not count proxy-only name resolution as guest acceptance.

## Repeated setup and service activation

Provisioning waits up to 30 seconds for the guest systemd manager before changing
service state. A failed reload is not retried. A changed companion/unit restarts
the DNS service; unchanged verified files use systemd start, preserving an active
service and starting an inactive one. Readiness, reload, enablement, active-state,
resolver, ownership, peer-identity and Policy checks remain mandatory.

Installed repeated-setup acceptance passed at `226991b`. Earlier manager-readiness
and start-limit failures remain in [acceptance evidence](../status/acceptance-evidence.md#development).
This success does not establish VPN/NRPT or OS-restart DNS propagation.

## Select the resolver when creating an Environment

`haco env create --workspace managed:work --dns host dev` uses the Physical Host
resolver. `--dns backend` selects the runtime adapter's normal resolver; on Incus
this is the owned trusted tooling instance. `--dns disabled` stops the managed
guest resolver and refuses managed lookups in the controller. Omission selects
`host`. No mode grants outbound connections or bypasses Policy and audit.

The setting is immutable for an existing creation and remains visible as
`dns_mode` in JSON Environment status. Snapshot, copy and export/import preserve
it; imported metadata never carries source approvals. Use a newly created or
copied Environment for another configuration. Unknown modes fail before creation.
Source identity is checked before and after resolution. Backend mode has no
fallback to Host or public DNS. The provider owns the bounded trusted operation;
Standard has no Incus-specific branching. See [ADR0094](../adr/0094-environment-resolver-selection.md).

Mode selection is implemented on main. Native provisioning, stop/resume and backend
resolution for all three modes passed on the supported Incus 7.0.1 baseline with
installed product companion `99522ebd`; this is not end-to-end guest Policy-query
or VPN/NRPT/restart acceptance. See [acceptance evidence](../status/acceptance-evidence.md#supported-dns-modes).
