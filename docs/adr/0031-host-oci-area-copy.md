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

Immediately before preparing the copy journal, the backend revalidates the Host
source readiness marker and managed daemon configuration. Setup-time validation
alone is insufficient after a later configuration change. Failure leaves the Host
running and does not issue pause/copy/resume commands; the canonical resource
reservation still remains available for recovery.

Before pausing, one provider configuration PATCH records the destination ownership
token and previous autostart setting, and disables autostart. The copier verifies
that durable guard, pauses the running Host and verifies the frozen status plus
unchanged ownership/attachment before copying. Ordinary Host entry refuses an
unfinished copy marker. A pre-existing paused Host or marker is never adopted.

This pause suspends Host processes; it is not a graceful Docker/containerd shutdown
or a saved running-container migration. It offers a filesystem snapshot boundary;
application crash recovery and usable image metadata require actual runtime
acceptance. The real fixture now proves built-image recovery for Docker
28.5.2/vfs and nerdctl 2.3.5/containerd 2.3.3/native after this pause/COW protocol.
It does not prove clean application shutdown, running-container migration or
all runtime versions/drivers.

Only positive copy completion permits resume. After verified running status, the
copier restores autostart and removes its marker, then verifies both. Any uncertain
copy outcome retains the marker, disabled autostart and source reservation instead
of resuming in `defer`. Failed resume/marker cleanup stays recovery-required. Receipt-bearing completed copies can now finish source restoration through
[ADR 0033](0033-completed-copy-recovery.md). Unconfirmed operations still require
recovery; editing away the marker or forcing Host start is unsupported.

## Fresh Host storage binding

Ordinary Host setup now creates the source through the canonical resource
lifecycle and attaches its exact owned volume at `/var/lib/hacocoon-oci`.
Preparation refuses existing data, symlinks, non-directory roots and existing
runtime configuration. It does not hide or migrate a populated default root.
The fresh Host receives containerd/Docker data roots in that area; `/run`
remains outside. Repeat setup verifies ownership, attachment, readiness marker
and bounded daemon configuration instead of overwriting them. Failed preparation
retains the creating source for recovery. Existing-data/custom-layout migration and optional runtime installation remain
incomplete. Environment setup now supplies the Docker managed roots while
preserving unrelated options and refusing conflicts or existing default data.
Owned Host nesting is enabled by the maintained setup integration under
[ADR 0032](0032-owned-host-nested-runtime.md). Actual runtime image recovery
remains a separate acceptance requirement.

## Rejected alternatives

- Image enumeration, tag selection and export/import rebuild a different storage
  area and do not provide the requested fast area-level copy.
- A synthetic or archive-populated source cannot prove copying the actual Host
  area, even if the final volume copy uses Btrfs.
- Copying active daemon state cannot establish a consistent stopped source.
- Sharing a writable root, credentials or management sockets violates isolation.
- Deleting ownership after merely attempting cleanup loses recovery authority.

## Acceptance

The real owned-area fixture now builds and executes images inside Host before
copying its area. A separate networkless instance with compatible runtimes keeps
the same image IDs and runs with `--pull never`; copy-image deletion leaves Host
images usable. The fixture also verifies Btrfs ancestry and bidirectional area
mutation/deletion, then removes its owned instances, volumes, Base, project and
pool. Docker 28.5.2 uses vfs; nerdctl 2.3.5/containerd 2.3.3 uses native snapshots.
This is provider/runtime acceptance, not full installed CLI recreation or proof
for arbitrary drivers, active tasks, runtime upgrades or interrupted copies.

The first actual-runtime run exposed missing Environment Docker data-root
configuration. The configuration now preserves other options while refusing
conflicts, unsafe files, active Docker units and existing default-root data.
A corrected fresh dedicated WSL run passed all fixture checks and cleanup.
The maintained GHA Btrfs job enables the same pinned-runtime extension.
Existing-data migration, opt-out/recreation combinations with real OCI tools and
unconfirmed-operation recovery remain required follow-up acceptance.
