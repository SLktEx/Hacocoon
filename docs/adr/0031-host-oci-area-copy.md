# ADR 0031: Copy the actual Host OCI storage area

Status: accepted requirement; Host quiescence integration planned
Date: 2026-09-08

## Decision

B4 copies the actual locally used Docker/nerdctl image storage area with Btrfs
snapshot/reflink semantics. It does not select individual images or rebuild an
image-only store by export/import. Compatible Environment runtimes must see the
same locally available image set after the independent copy.

The existing managed resource catalog, exact provider ownership and volume COW
copy remain the storage foundation. Host source integration must stop all writers,
prevent restart while copying and prove the owned source's quiescence. Socket
activation, cancellation, concurrent inverse operations and controller failure
are part of this protocol, not reasons to copy an active daemon directory.

Only the designated image storage area and necessary runtime metadata are copied.
Process state, management sockets, reusable Host credentials and unrelated Host
volumes stay outside it. The source remains Host-owned. Every copy gets its own
identity and writable area; workload-used data never returns to Host. A retained
Workspace Store wins over copying again on Environment recreation.

Use the canonical durable resource lifecycle. Record ownership before mutations;
uncertain copy/cleanup retains source and destination until the operation is
known quiescent and the owned object is absent. Source writer restoration must
not race an unfinished copy. A future recovery path must handle process death;
`defer` cleanup alone is insufficient. Current attached-source rejection remains
until the provider has verified quiescence support.

## Rejected alternatives

- Image enumeration, tag selection and export/import rebuild a different storage
  area and do not provide the requested fast area-level copy.
- A synthetic or archive-populated source cannot prove copying the actual Host
  area, even if the final volume copy uses Btrfs.
- Copying active daemon state cannot establish a consistent stopped source.
- Sharing a writable root, credentials or management sockets violates isolation.
- Deleting ownership after merely attempting cleanup loses recovery authority.

## Acceptance

The Host producer is incomplete. Existing synthetic Btrfs volume tests prove
storage COW and independent writes only. Required real runtime acceptance covers
both Docker and nerdctl: images prepared in Host, automatic Environment creation,
unchanged local identities without registry access, independent source/copy
changes and deletion, opt-out and recreation. Add deterministic failure/restart
regressions at the lifecycle layer and actual stopped-writer/copy checks in E2E.
