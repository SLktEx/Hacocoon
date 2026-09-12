# ADR 0039: Preserve ownership through snapshot restore preparation

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/environment-snapshots.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Base retention and automatic backup decisions below are historical where
superseded by [ADR 0040](0040-incus-first-snapshots.md).

Status: accepted design; internal preparation catalog implemented; service/provider staging implemented; lifecycle replacement pending
Date: 2026-09-08

## Decision

Restoring saved work must preserve the current work before any replacement.
The service will hold the canonical Environment and Workspace locks, verify a
stopped target, capture a fresh pre-restore snapshot, then reserve a complete
independent destination plan. Source snapshot, pre-restore snapshot and every
new component identity are recorded together before provider copying. Each
successful create receives a durable receipt before verification. Every component
must verify before preparation is complete.

A prepared operation is not a runnable Environment and cannot publish replacement
metadata or release current ownership. A later canonical lifecycle transition
must perform the actual replacement, connection/network reconciliation and
recovery. Orchestration must not independently change Environment metadata and
Workspace leases. The pre-restore snapshot remains available after success.

All preparation states, including cleanup, keep both snapshots reserved and
block conflicting lifecycle operations on the current Environment. Interruption,
failed publication and cleanup retain exact identities. Cleanup removes only the
new copies and releases the reservation only after every one is positively absent.
It does not delete the source or pre-restore snapshots, or change the current
Environment. Snapshot deletion and restore reservation are one atomic catalog
transaction, not a read-then-check convention.

Schema 8 persists this operation separately from immutable snapshots. It reads
older catalogs including schema-7 Base assets, but refuses older schemas claiming
restore records. Both saved manifests and the current Environment creation ID
must still match. Legacy ownership-only snapshots with no provider bindings
cannot be restore sources.

## Rejected alternatives

- Replacing current data before creating recoverable pre-restore material.
- Relying only on process-local locks or expiring a crashed restore reservation.
- Deleting a snapshot after a separate observation that it appears unused.
- Treating prepared copies as a completed restore or starting a partial aggregate.
- Releasing ownership because deletion was attempted or a create returned an error.
- Restoring Host credentials, controller Policy or protected management identity.

## Current scope

The internal catalog and lifecycle/deletion guards are implemented and tested for
restart, complete receipts, binding drift, downgrade, partial cleanup and races.
Fresh pre-restore capture and provider staging are now implemented through the
canonical locked service. Incus copies each saved component into independently
owned stopped/unattached storage, clearing source configuration and management
identity while preserving required idmap bookkeeping. Saved and destination
provider routes must agree. Source ownership is rechecked before copy and target
ownership before verification or cleanup. Lost replies retain the planned target.
Replacement, restore execution and public CLI are still required; prepared storage
is not a completed restore. Tests of the catalog alone are not real-host restore
acceptance. See [snapshot design](../design/environment-snapshots.md) and
[canonical lifecycle](0002-environment-lifecycle-ownership.md).
