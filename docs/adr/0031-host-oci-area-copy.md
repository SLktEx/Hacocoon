# ADR 0031: Copy the actual Host OCI storage area

Status: accepted; provider pause/copy/resume slice implemented; full Host area integration partial
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
`defer` cleanup alone is insufficient. Attached-source rejection remains except for the exact owned Host area under the
provider protocol below.

## Provider pause and restart guard

The Incus copier now recognizes only source-only `oci-source:host` with one exact
Host consumer and one matching disk at `/var/lib/hacocoon-oci`. It verifies the
Host role, volume ownership and expanded attachment configuration. Other attached
sources remain refused. The canonical catalog reserves source and destination
before this provider action begins.

Host start and the attached-area copy also share a Linux cross-process lock,
implemented by an exclusive abstract Unix socket bind per project. No commands
or state are served on that socket. A process crash releases the lock, while the
persistent copy marker still blocks restart. The marker check and start happen
under this lock so a concurrent start cannot pass the check before pause and
then thaw the Host during the copy. Unsupported platforms fail closed.

Before pausing, one provider configuration PATCH records the destination ownership
token and previous autostart setting, and disables autostart. The copier verifies
that durable guard, pauses the running Host and verifies the frozen status plus
unchanged ownership/attachment before copying. Ordinary Host entry refuses an
unfinished copy marker. A pre-existing paused Host or marker is never adopted.

This pause suspends Host processes; it is not a graceful Docker/containerd shutdown
or a saved running-container migration. It offers a filesystem snapshot boundary;
application crash recovery and usable image metadata require actual runtime
acceptance. That acceptance is still missing. Do not claim clean application
shutdown or complete OCI distribution from the provider tests.

Only positive copy completion permits resume. After verified running status, the
copier restores autostart and removes its marker, then verifies both. Any uncertain
copy outcome retains the marker, disabled autostart and source reservation instead
of resuming in `defer`. Failed resume/marker cleanup stays recovery-required. A
complete operator recovery implementation is still required; editing away the
marker or forcing Host start is not a supported recovery procedure.

## Fresh Host storage binding

Ordinary Host setup now creates the source through the canonical resource
lifecycle and attaches its exact owned volume at `/var/lib/hacocoon-oci`.
Preparation refuses existing data, symlinks, non-directory roots and existing
runtime configuration. It does not hide or migrate a populated default root.
The fresh Host receives containerd/Docker data roots in that area; `/run`
remains outside. Repeat setup verifies ownership, attachment, readiness marker
and bounded daemon configuration instead of overwriting them. Failed preparation
retains the creating source for recovery. Existing-data/custom-layout migration
is still incomplete, as are runtime installation and Docker Environment setup.
This does not enable Host nesting or prove actual runtime image recovery.

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

The dedicated local Incus/WSL provider E2E passed actual pause, attached-volume COW,
resume, Btrfs ancestry and bidirectional mutation/deletion independence with full
fixture cleanup. It uses synthetic data and does not prove Docker/containerd
application recovery. The maintained GHA Btrfs job runs the same provider fixture.
