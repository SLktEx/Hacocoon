# Trusted Host Packer with nested Incus

Status: accepted; replaces [ADR 0089](0089-guest-packer-provisioning.md).
[日本語](0114-trusted-host-nested-packer.ja.md)

Packer/HCL/plugins are trusted build code and execute in `haco-host`.
The native Incus plugin creates instances/images in a fresh owned project of a
Host-local nested daemon. Reuse the existing trusted Host nesting policy and
signed Incus LTS installer. Do not expose Physical Host sockets, storage or state.

Refine ADR 0032 for new Hosts: configure nesting before the first boot. Incus
initializes nesting mounts and AppArmor namespaces during startup; a live config
update alone is insufficient for a fresh nested daemon. OCI source verification
still precedes tooling setup. Do not disable AppArmor or use a privileged Host
to compensate for an incorrectly initialized instance.

Transfer only the native container archive through canonical Base import.
Physical Host code validates bytes and owns publication; it has no HCL interpreter,
Packer execution adapter or second Base catalog. JSON builds remain ordinary Env
provisioning. Preserve immutable revisions and the current Base publication contract.

A durable build receipt records project/image/artifact/import identities before
further fallible work. Verify plugin cleanup independently. Clean nested resources
before upload; retain unknown outcomes without automatic replay or guessed deletion.
A lost commit acknowledgement is uncertain, not proof of rollback. Fully published
revisions survive subsequent cleanup failure.

Rejected: Physical Host HCL execution, ordinary-Env management authority, the
historical same-Env null/SSH adapter, a duplicate Packer publication/catalog, and
success inferred from mocked execution requests. Require real plugin/download/build,
native export/import and new-Env tool execution in the maintained CI gate.
See the [contract](../design/packer-base-builds.md).

Build/import size and overall duration have no configured cap by default, per operator choice. Optional explicit image caps, bounded-memory streaming, cancellation, finite CPU/memory/PID budgets and archive validation remain. This does not promise infinite filesystem capacity or TB-scale acceptance. Abrupt CLI loss can leave the exact worker service running; retain its receipt and inspect/stop that service without guessing resource cleanup.
