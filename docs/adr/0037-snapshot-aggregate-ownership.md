# ADR 0037: Snapshot the complete owned Environment aggregate

Status: accepted design; source guard and durable catalog implemented; capture/restore planned
Date: 2026-09-08

## Decision

A snapshot describes Environment rootfs, all managed Workspace/Git members and
the optional exact persistent OCI attachment as one logical aggregate. A rootfs
snapshot alone must not be presented as saving all work. Trusted Host authority
and controller configuration are excluded.

The first supported configuration is stopped, managed storage. External host
directories are refused instead of copying them while unobserved writers can
change data. Environment and Workspace lifecycle locks span source validation
and eventual capture. Exact persisted creation identity prevents a recycled name
from becoming the source or restore target.

An inspection result is not a durable reservation or completed snapshot.
The schema-5 owned component catalog reserves capture before mutation and blocks
source start/delete across restart until publication or proven-absent cleanup.
Provider integration must record exact created resources immediately and preserve
ownership on ambiguity
as required by ADR 0002. Restore cannot silently discard the current working state.

## Rejected alternatives

- Calling a runtime-only snapshot a complete Environment/Workspace save.
- Inspecting while locked, then releasing locks before source mutation.
- Treating stopped guest state as proof that external host data is quiescent.
- Releasing component ownership merely because cleanup was attempted.
- Restoring Host credentials or management state from workload data.

## Current evidence

Internal race tests validate the source guard and concurrent delete exclusion.
No provider capture or restore is implemented by this decision's first change.
See [snapshot design](../design/environment-snapshots.md).

The catalog uses a new schema version so older controllers reject it rather than
rewrite the file without snapshot reservations. Ready components must survive
source deletion; instance-bound snapshots alone cannot satisfy that contract.
Catalog restart, transition, cleanup and concurrent-reservation regressions pass;
provider capture and restore remain planned.

Capture and delete now have one Workspace-service coordinator enforcing the
reservation/receipt/verification sequence under the canonical lifecycle locks.
A backend supplies only plan/create/verify/proven-absent deletion; it cannot
publish the catalog or release source ownership. Production Incus integration
and restore remain planned.
