# ADR 0015: Reserve offline persistent resources during independent copy

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/persistent-oci-store.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted
Date: 2026-09-07

## Decision

Extend ADR 0014's persistent-resource lifecycle with an offline copy transition.
The existing `haco plugin oci store create <target> --from <source>` operation
creates another persistent Store. No new command group, required argument for
ordinary creation, Environment lifecycle API, Host runtime or Core OCI dependency
is introduced.

The controller atomically records a new target owner/native identity and its
exact `copy_source` reservation. The source must be ready and have no Environment
or lease, including stopped Environments. Canonical Environment creation and
explicit Store deletion reject a reserved source. The creating target cannot be
attached or deleted. The provider validates source identity and absence of
attachments again, then creates a same-pool copy with new ownership markers.
Only successful provider completion, target verification and a durable commit
release the source reservation and publish the target as ready.

Incus owns the Btrfs operation. The adapter verifies the Btrfs driver, requests
volume-only copy, preserves the source's validated idmap bookkeeping and does
not inherit arbitrary configuration or mount the source in trusted Host. Copied
Store contents remain untrusted Environment data. A copy may be attached to a
different Environment after publication; subsequent writes and deletion have
independent lifetimes.

## Failure and recovery

A provider timeout/cancellation can leave an asynchronous copy operation active.
An absent destination alone therefore does not prove completion. A failed copy,
verification or commit retains both exact identities and the source reservation
across controller restart. Duplicate destination creation, source attachment,
source deletion and target deletion are refused. Inspect exposes `creating` and
`copy_source`. No automated recovery is provided in this slice: an administrator
must establish provider operation quiescence and exact ownership before any
future recovery transition can release the reservation. Never edit the catalog
or delete a volume as a routine workaround. Successful copies need no recovery.

## Rejected alternatives

- Reusing or simultaneously mounting one writable Store does not provide copy
  independence and permits concurrent runtime metadata corruption.
- Copying an attached/stopped Environment's Store assumes runtime quiescence
  without a lifecycle reservation or positive provider evidence.
- Releasing the reservation on command timeout, a failed validation or apparent
  destination absence can race the still-running provider operation.
- Mounting guest-populated Store data in trusted Host crosses the trust boundary.
- Copying a Host live Docker/containerd root can include credentials, active
  process state and authority; it is not the requested distribution contract.
- Whole-file save/load bypasses the persistent-resource/COW ownership model.

## Remaining work

This is the independent-copy foundation of revised B4. Trusted Host image
acquisition into a credential-free publishable Store, Docker support, complete
OCI image/runtime acceptance and bounded interrupted-copy recovery remain
partial/planned. They must preserve these ownership and trust invariants.
