# Environment resolver selection

Status: accepted; implemented in the development candidate. [日本語](0094-environment-resolver-selection.ja.md)

## Decision

Core defines per-Environment `host`, `backend`, and `disabled` name resolution.
The default is `host`: the Physical Host platform resolver, including normal
WSL DNS tunneling. It is not the trusted logical Host's resolver configuration.
The replaceable Standard resolver selects from current creation-bound metadata
after the existing Policy and audit path. Disabled queries cannot reach either
upstream. Unknown modes or mismatched ownership fail closed.

Backend resolution remains behind the provider seam. The Incus adapter uses the
verified trusted tooling instance's normal resolver path for this explicit mode,
with bounded fixed code and literal query data. It never enables a shared bridge
DNS listener or exposes raw DNS egress to the untrusted guest. The trusted tooling
resolver is not substituted for the Physical Host in `host` mode.

The guest's existing guarded DNS relay remains the only managed request path.
Disabling its local service is a usability measure; controller-side refusal owns
the security boundary even if a guest runs its own stub. Successful DNS still
grants no HTTP, TCP, UDP, Host-service or peer-Environment connection authority.
Changing or recreating an Environment must not reuse another creation's mode.
Copy, snapshot and transfer retain this setting without copying approvals.

## Rejected alternatives

Copying resolv.conf does not preserve platform resolver semantics. Enabling
Incus dnsmasq for all guests would widen the existing managed-network boundary.
A guest-selected upstream or arbitrary trusted-Host command would create an
unrestricted resolver/control path. Backend code and its platform details do not
belong in Core or in the Standard provider's platform conditionals.

See [name resolution](../design/name-resolution.md). Actual mode provisioning,
transfer and installed acceptance must be recorded before claiming completion.
