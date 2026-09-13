# Portless SSH through controller sessions

Status: accepted design for Issue #629; native acceptance remains separately
recorded in [acceptance evidence](../status/acceptance-evidence.md).

## Decision

Ordinary SSH uses OpenSSH ProxyCommand stdio through the existing local controller
UDS and generic byte-session transport. The Incus adapter opens a socket in the
verified Environment network namespace using its existing generation-pinned
network dialer. A provider PID resolving to the controller's Host network
namespace is rejected before dialing. Environment sshd continues to listen on port 22.

The desktop stores a stable human-readable `haco-<name>` alias. Its encoded target
binds the Environment creation ID, Workspace ID, access mode, named service and
persistent SSH grant. Display names and WSL distribution names grant no authority.
The controller resolves provider references from its catalog, validates the active
lease and provider generation, and checks the grant before resuming a stopped Env.
The same lifecycle lock excludes create/delete/stop and simultaneous reconnects
through socket acquisition. Sessions release that lock before relaying traffic.

WSL starts through its normal `wsl.exe --distribution ... --exec` entry. Read-only
controller readiness is bounded to two minutes; target preparation is separately
bounded. Cold reconnect never installs, repairs, changes permissions or Policy,
creates an Environment, or adopts another Workspace. Missing, revoked, mismatched,
unknown or recovery-required targets fail closed.

## Access and lifetime

SSH grants live in controller-owned Incus configuration and survive stop/reboot.
A pending grant is durably recorded before public-key installation. Only a fully
validated host-key result can publish a ready grant. Revocation disables the grant,
removes its managed public-key marker, then removes grant metadata. A controller
disconnect closes every live stream for that exact generation/grant under the
lifecycle lock; other grants remain connected. Ambiguous
cleanup retains evidence and refuses streams. Guest data cannot publish authority.
Private keys stay on the client; host keys remain pinned with strict checking.

Byte sessions reuse the controller session's separate completion and cancellation
identity. Protocol handshakes are consumed by the client before exposing bytes to
OpenSSH. No frames, progress or JSON are written to ProxyCommand stdout. EOF
half-closes the opposite writer; cancellation closes both sockets and joins copy
workers. An abandoned half-closed session has a bounded 30-second drain. The
controller connection limit also bounds simultaneous relays.

The existing managed Include and confined atomic file writer own only Hacocoon
fragments. Explicit disconnect/deletion and `haco ssh cleanup` remove identified
fragments, preserving user configuration, shared identity and host-key pins.
Conflicting user aliases and aliases from another WSL installation are refused.

## Rejected alternatives and migration

Host ephemeral SSH listeners, port reservations and SSH proxy creation are removed.
The removal-only `ssh_migration.go` handles already installed legacy proxy devices:
verify their owned shape, revoke the old managed key, remove the device, and prepare
a new portless grant. It is migration code, never a reusable transport pattern.
General user-requested forwarding and preview retain their separate contracts.

A Windows controller TCP server, direct client Incus access, a second SSH-specific
transport, a mandatory VS Code extension and name-based recreation on reconnect
would weaken existing authority or duplicate established mechanisms. They are not
part of this design. SSH grant provisioning retains the existing management UDS
authorization boundary; network Policy/approval and guest egress are unchanged.

See [client access](../design/client-and-interactive-access.md),
[controller transport](../design/controller-client-transport.md) and
[lifecycle ownership](0002-environment-lifecycle-ownership.md).
