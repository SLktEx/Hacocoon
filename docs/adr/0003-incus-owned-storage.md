# ADR 0003: Incus owns local Btrfs storage

Status: accepted, 2026-09-05.

## Context

The WSL path previously had two owners for storage startup: Hacocoon prepared raw files, loop devices and mounts, while Incus consumed an external source path. Mount visibility and service startup could diverge. Retaining the old implementation in installation, release payloads and CI also made it possible to select that rejected architecture again.

## Decision

The local Incus adapter requests `haco-local-default` as an Incus-owned Btrfs pool with `size=128GiB`, without supplying `source=`. Incus owns the backing image, loop, formatting and mount lifecycle. Hacocoon owns desired pool selection and `compress=zstd:3,noatime,nodiscard` policy, reconciled through Incus with readback. See the [storage contract](../design/btrfs-storage-layout.md).

The removed Host storage implementation, privilege executable, selection switches, release payload and dedicated acceptance workflow must not return. Existing Incus CI owns the storage acceptance job. Linux and WSL keep the same ownership boundary.

Runtime attachments carry an existing `incus_pool` identity only. The runtime rejects the former `driver`/`source` shape even if a pool already exists, and never creates an external-path replacement when inspection fails.

## Consequences

No arbitrary Host mount authority is added to Core, the controller client or `haco-host`. The Physical Host controller retains management state and provider authority. Installation does not delete or convert existing user data; unsupported old storage layouts have no migration contract. Repeating a current installation must preserve the Incus pool and its data.

Configuration readback and live mount acceptance are distinct. Failure to apply policy must surface as failure; a Host-side remount is not an acceptance workaround. Seed removal is a separate change and does not remove Base selection or optional Plugin contracts.

## Rejected alternatives

- Keeping the old privileged storage executable as a fallback or test-only product mode: restores a second lifecycle owner and a second privileged surface.
- Repairing Incus with direct remounts, loop attachment or service-order tricks: bypasses the storage owner and hides startup defects.
- Reformatting/recreating a populated pool on reinstall: destroys current user work.
- Reintroducing a Windows native launcher: unrelated to storage ownership; Windows entry remains WSL.
