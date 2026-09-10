# Environment transfer

Status: **planned** for the public export/import flow. The native rootfs/volume
acceptance tests are internal prerequisites, not a usable Hacocoon importer.

## Incus foundation

Use native Incus archives for rootfs and custom volumes. Rootfs can be transported
as a single published image archive; custom volumes use volume export/import.
An instance archive does not contain attached Workspace or OCI custom-volume
contents. Hacocoon must bind those archives to their intended roles and check that
all required components were saved before reporting a complete export. Base names
and fingerprints are provenance only; no extra Base filesystem component is needed.
See [instance backup scope](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/)
and [custom-volume backups](https://linuxcontainers.org/incus/docs/main/howto/storage_backup_volume/).

Ordinary file archives can move between pools. Incus optimized archives are tied
to a compatible storage driver; do not treat this difference as merely speed.
Neither successful optimized export nor a new snapshot is a prerequisite for the
later damaged-storage evacuation workflow. That workflow remains separate.

## Data and authority

A future public import creates new managed resources through canonical lifecycle
ownership, preserving Workspace Git state and retained OCI data. It must not
replace an existing Environment or restore old approval, connection or management
authority. Host credentials and control sockets are outside Environment export.
An archive checksum detects changed bytes; it is not authorization to apply its
configuration. Imported owner labels are source metadata, not a newly issued lease.

The dedicated Incus 6.0.5 CLI has no instance-import config/device override flags,
although the current upstream documentation describes them. Do not implement
import by assuming newer flags or by starting an instance before replacing its
old configuration. Rootfs archive validation, current security reconstruction,
file-path handling and aggregate publication still need implementation and tests.

## Native custom-volume acceptance

`TestRealIncusVolumeTransferE2E` is opt-in with
`HACO_E2E_INCUS_VOLUME_TRANSFER=1` on a dedicated root Linux/WSL Incus+Btrfs host.
The runner must see the Incus storage mounts; for a daemon with a private mount
namespace, enter its freshly observed namespace within the same invocation.
It uses two new random pools and synthetic `work`/`oci` volumes. It writes an exact
ownership plan under `/var/lib` before creation and retains the plan and archives
outside both pools. Failure retains resources for explicit inspection. Cleanup
checks the test ownership marker and positive pool emptiness before pool deletion.
It never selects an existing Workspace, OCI Store, snapshot, shared image or pool.

The test uses non-optimized `--volume-only --compression=none` export and import
into the second pool. It checks unpushed Git commits, uncommitted and untracked
files, hardlinks, symlinks, file mode, independent writes, archive immutability and
source-deletion independence. It also checks that native import carries the old
user-config marker, making the need for fresh Hacocoon ownership explicit.

Public rootfs import, UID/GID and extended-attribute coverage, real Docker/containerd
contents, public commands, Windows artifact delivery and cross-host acceptance
remain unverified. A passing volume test does not prove the complete G1 flow.

The first dedicated run failed before export because the fixture omitted the
`default_` volume path prefix and ran outside the daemon mount namespace. Its
exact owned resources were retained, then deleted after matching owner markers
and empty inventories. The corrected run inside the current daemon namespace
passed in 11.24s on Incus 6.0.5/Btrfs. Both test pools were cleaned; the exact plan
and two archives remain at `/var/lib/haco-volume-transfer-2481101147`. The original
failed-run plan remains at `/var/lib/haco-volume-transfer-3035986437`.

## Native rootfs image acceptance

`TestRealIncusRootfsTransferE2E` runs with `HACO_E2E_INCUS_ROOTFS_TRANSFER=1`.
It creates a fresh isolated project/pool and an empty stopped instance without any
Base or cached image. Incus publish/image export saves one rootfs archive. The
source instance and published image are deleted, and the project image inventory
is positively empty before import. Creating a new instance from the imported
image uses explicit current config and `--no-profiles`: the old environment token,
instance ID and profiles are not restored. Guest file bytes and archive checksum
are checked. Exact marker checks protect cleanup; the archive and plan remain.
This is the rootfs component itself, not an additional Base filesystem, Base
registration or renamed Base retention object. Existing snapshot copies are unchanged.

The dedicated Incus 6.0.5/Btrfs run passed in 14.88s. Its retained archive is
`/var/lib/haco-rootfs-transfer-2526041618/rootfs.tar`, SHA-256
`9df946096ec6eb1b2a7a999937dd1d78bf79b61db91ebea43a6310f0843abf78`.
The initial run failed on the fixture's unsupported `image get` command. It was
changed to the existing JSON image API; exact owned leftovers were removed after
marker checks, while `/var/lib/haco-rootfs-transfer-430939700/plan.json` remains.

The extended dedicated WSL Incus/Btrfs test passed in 22.44s: after deleting
the imported source image, the image inventory is empty and a fresh file pull
from the stopped destination still matches its pre-deletion bytes. The retained
archive checksum also remains unchanged. This checks native image/rootfs
independence, not deletion of container-referenced OCI images.

This tiny data fixture is not bootable-OS, SSH, managed network, credential,
template or aggregate-import acceptance. Public import still needs canonical
ownership/creation, archive validation and current connection/security setup.
Published/imported image properties and profile associations never grant authority;
callers must use the current configuration explicitly. No public command or new
catalog schema is introduced by these tests.

## Internal transfer envelope

`internal/environmenttransfer` implements streaming `Write` and read-only `Inspect`.
The internal version-1 manifest contains a bounded source label, OCI presence and
an ordered list of role/size/SHA-256 records. The outer USTAR has fixed regular
entries: `manifest.json`, `rootfs.tar`, `workspace.tar`, additional numbered
`workspace-002.tar` through `workspace-253.tar`, and optional `oci.tar`.
The Workspace count follows the current Incus snapshot attachment bound; every
number must be consecutive. Metadata is bounded to 64 KiB and total envelope
overhead to 512 KiB, independently of the caller-owned payload budget.
There is no Base component, provider path, source management ID or credential map.
This is not yet a published interchange format or a public export/import command.

Canonical bounded JSON rejects duplicate/unknown fields. Required role/order,
sizes, caller-owned aggregate budget, header type/format, payload hashes, complete
closing blocks and absence of trailing content are checked. Reads are bounded
even before manifest parsing, including tar's hidden extended-header processing.
Neither function extracts files, calls Incus or invokes a consumer before whole
verification. Native inner-archive safety and source authenticity are not proved.

`WriteSnapshot` compares archives against the entire protected ready-snapshot
inventory, sorting Workspace roles for deterministic transport order and requiring
exact component identity, binding and state. It rejects missing, extra, duplicate
or mismatched components before output, including omitted Workspace/OCI archives.
Legacy Base records remain in the source catalog but need no archive or Base
filesystem lookup. Unknown retained roles are rejected rather than silently omitted.
The caller must hold canonical source reservations and verify native ownership
while creating archives. A supplied snapshot object is not itself authority; this
function does not replace lifecycle locks or native checks. `Inspect` alone cannot
discover source data omitted from an untrusted manifest.
Only a successful writer result may be published. Any failed output remains
unpublished, with cleanup ownership retained if removal is uncertain. Later import
must keep immutable staged bytes or reverify them, then use canonical lifecycle
and current security settings. Verification does not authorize source labels or
old configuration. See [ADR 0049](../adr/0049-transfer-envelope-authority.md).

Focused race tests and vet passed. The first truncation regression failed because
Go's tar reader permits EOF without closing blocks; explicit closing-block
accounting corrected it. Regression coverage includes missing/changed payloads,
missing OCI, Base/extra/reordered components, path/link/header attacks, duplicate
JSON, over-budget and overflowing sizes, and bounded pre-manifest reads.

## Internal Linux staging

`Stage` is **implemented** for Linux/WSL. It opens a controller-owned private
staging directory without following symlinks, copies bounded input into a native
`O_TMPFILE`, reopens that same inode read-only and closes the writable handle
before whole-envelope verification. Only verified bytes and copied metadata are
returned. Readers do not expose a file descriptor or pathname. Replacing the input
or staging-directory path cannot replace those bytes. Host administrators with
process-descriptor authority remain outside this guarantee.

Closing the result or exiting the process releases the unnamed file. Failed input
leaves no named artifact or Incus resource. This is process-lifetime staging, not a
durable recovery record or automatic backup. Unsupported `openat2`/`O_TMPFILE`
filesystems fail explicitly; there is no fallback or permission repair. The caller
must cancel its transport to unblock a pending source read.

Linux real-filesystem race tests and vet passed, including path replacement,
read-only handles, malformed/oversized input, maximum integer budget and transport
failure after complete bytes. An initial validation invocation failed before tests
started because of shell PATH quoting; the corrected invocation passed. Btrfs
staging and public import integration remain unverified. Earlier native Incus
archive acceptance does not establish those claims.

The inventory regression tests exercise all 253 supported Workspace components,
with and without OCI, independent ordering, missing/mismatched components and
legacy Base exclusion without catalog mutation. These are repository tests;
multi-Workspace native aggregate export remains unverified. The earlier single-
Workspace-only internal envelope was never a public format or catalog schema.
Its original single-Workspace encoding remains readable, and no saved data migrates.

## Saved source lifetime during export

`workspace.Service.ReadSnapshot` is **implemented** as the source-use boundary.
It shares the existing Environment-then-Workspace lifecycle lock path with
`DeleteSnapshot`, reloads the catalog after locking and rejects a changed source
identity or non-ready save. Every retained rootfs/Workspace/OCI component is
verified through the existing runtime adapter before the consumer runs. Historical
Base filesystem components are not looked up. The original Env need not exist.

The consumer must finish reading all source data before returning, and must not
re-enter lifecycle operations. Keeping a returned snapshot value is not a
reservation. Cancellation and consumer/verification failure release process locks
without altering the save. Linux/WSL uses the existing cross-process filesystem
locks; the non-Linux test implementation remains process-local, not a new native
controller platform. No backup, durable export state or automatic replay is added.

This boundary is not yet wired to native archive production or a public export
command. Its source locks do not replace exact temporary-resource ownership,
complete output publication or destination security reconstruction.

## Native saved-volume export adapter

`incus.Runtime.ExportSnapshotVolume` is **implemented** on Linux/WSL for a saved
Workspace or OCI volume. It decodes the existing protected component binding and
verifies detached native ownership before and after ordinary, volume-only Incus
export. The caller still holds `ReadSnapshot` throughout use. This method does not
export rootfs or publish a complete Environment bundle.

The CLI writes into an unnamed local file through the live parent process's
`/proc/<pid>/fd/<fd>` path. The writable descriptor is closed before hashing and
returning read-only bytes. No archive bytes pass through the bounded stdout logger,
no path is returned to consumers, and failure leaves no named local archive. The
private controller directory must support `openat2` and `O_TMPFILE`; unsupported
systems fail explicitly. The size budget is checked after native materialization,
not a disk quota while Incus writes. Host administrators with descriptor authority
are outside the file-replacement guarantee.

[Incus 6.0.5 volume export](https://github.com/lxc/incus/blob/v6.0.5/cmd/incus/storage_volume.go)
creates a temporary native backup and ignores errors from its deferred deletion.
The adapter compares backup names, creation/expiry times and flags before and
after export. New or changed remaining backups, or a failed final observation,
prevent a successful result. Errors identify the saved volume to inspect. Existing
backups are never removed by Hacocoon; a name alone is not cleanup ownership.
A failed native command remains a failure even if all output bytes were written.

Focused tests cover ownership changes, native errors after output, retained or
unobservable backups, reused backup names, empty/oversized output and actual child
process access to the anonymous file. `TestRealIncusOwnedVolumeExportE2E`, enabled
by `HACO_E2E_INCUS_VOLUME_TRANSFER=1`, uses one new owned Btrfs pool and synthetic
saved/imported volumes. It retains an exact plan and archive, checks source-deletion
independence, and deletes only identified fixture resources after verification.
The existing Incus GHA job includes this test. Public aggregate export/import,
public orchestration, OCI daemon contents and destination authority remain unfinished.

The dedicated Incus 6.0.5/Btrfs run passed in 5.92s. The retained plan and 4096-byte
archive are under `/var/lib/haco-owned-volume-export-778805159`; archive SHA-256 is
`33e2785b3d574d504fb104c4a16bd154339d1c675d43241418ab1ecc171d155f`.
The source and imported fixture volumes and their owned pool were removed after
checks. Local focused race tests and vet passed. Initial unused-import and string-
literal fixture build errors were corrected before native execution. This proves
the new single saved-volume adapter, not public aggregate transfer or OCI daemon
acceptance. Anonymous staging on a Btrfs destination filesystem remains unverified.

## Native saved-rootfs export adapter

Status: **implemented internally; dedicated native adapter acceptance passed**. Public G1 still
requires the stopped-Env export/import/start/SSH flow; users must not need to take
a separate snapshot merely to export an Env. This adapter is a prerequisite using
an already protected independent saved rootfs, not that public flow.

`ExportSnapshotRootfs` uses the official Incus 6.0.5 client in the Linux/WSL
Incus adapter. It verifies the saved component under the caller's `ReadSnapshot`
reservation, publishes a unified uncompressed image, streams its bytes into the
existing anonymous `NativeArchive`, verifies the source again and removes its own
transport image. The image is the rootfs component in transit, not an additional
Base filesystem. Source Base/image cache lookup, saved-source deletion, automatic
backup and restoration of old authority are not part of export.

The selected private Unix remote comes from Incus CLI configuration. The socket
and project are recorded with a random owner before publication; the returned
operation and fingerprint are durably appended before later checks. HTTPS remotes
and clustered daemons are explicitly unsupported for this initial local flow.
There is no silent local fallback. SDK metadata reads are limited to 1 MiB, event
listeners are disabled and operation waits must report terminal success.

The image endpoint uses the selected Unix transport and an explicit request
context. It rejects redirects and split images, bounds actual writes and checks
the full archive SHA-256 against the fingerprint. Response filenames never become
local paths. This avoids SDK 6.0.5 `GetImageFile`'s alternative `/dev/incus/sock`
attempt and lack of inherited download context. No archive is extracted or run.

A private `rootfs-export-<owner>.jsonl` receipt is retained if publication or cleanup
is uncertain. An ambiguous operation is not replayed automatically: inspect its
recorded socket/project/operation and identify images by the exact owner property
`user.hacocoon.export-owner`. Only a verified owned image with no aliases is
removed; positive absence must precede receipt deletion. A replaced or shared
receipt is not unlinked. The saved rootfs remains untouched. See
[ADR 0050](../adr/0050-native-rootfs-export-ownership.md).

Regression tests cover source/image owner changes, lost publication replies,
unfinished operations, failed downloads, digest/size failures, uncertain cleanup,
receipt replacement, split/redirect refusal and stalled-transfer cancellation.
`TestRealIncusSnapshotRootfsExportE2E` is in the existing opt-in GHA rootfs transfer
gate. It creates an isolated project/pool and synthetic saved rootfs, then checks
native export, temporary-image absence, saved-source preservation and native
archive re-import. It does not establish bootable public import, SSH or OCI data
acceptance. Native execution results must be recorded separately from unit tests.

The dedicated WSL Incus 6.0.5/Btrfs adapter run passed in 13.44s. The isolated
source/project/pool and imported transport image were cleaned after their checks.
The plan and archive remain at `/var/lib/haco-rootfs-export-1396608668`; archive
SHA-256 is `195bb069299c130f187cd1cb814806a8e39966f2b089a86fcb9460e5d3e8da85`.
Focused race regressions passed in 3.614s; package vet passed. This native result
covers this component adapter, not the unfinished public G1 workflow.

## Internal stopped-Environment export

Status: **implemented internally**, with public CLI/controller delivery still planned.
`environmenttransfer.Exporter.ExportStopped` uses the existing canonical
`CaptureStoppedSnapshot`, `ReadSnapshot` and `DeleteSnapshot` operations. It does
not stop a running Environment or require a separate user snapshot command. The
one temporary COW capture is the coherent export source, not a pre-restore backup.

The private output directory is opened before capture. Inside the saved-source
reservation, the full rootfs/Workspace/optional OCI inventory is validated before
native export, then every archive consumes the same aggregate byte budget. Legacy
Base filesystem records stay in the catalog and are excluded from transport.
All native handles are closed inside the reservation. Only this invocation's
canonical capture is deleted, using a bounded cleanup context even after caller
cancellation; uncertain deletion retains its exact snapshot ID and catalog state.

The envelope is written synchronously into the existing anonymous staging file.
Neither complete bytes followed by a producer/cleanup error nor an incomplete
aggregate can return a bundle. The final read-only file passes whole-envelope
verification before return. Source Env identity, leases, existing snapshots and
current data remain unchanged. There is no new catalog, crash replay, rollback
or automatic import/restore backup. Incus still supplies the native copy/export
behavior through its component producers; this package composes the data boundary.

Regression coverage includes all 253 Workspace roles with optional OCI, historical
Base exclusion, aggregate budgets, late failures, cancellation and descriptor
cleanup. A real JSON catalog/canonical-lifecycle test covers running refusal,
shared deletion locks, retained snapshots and uncertain-cleanup records with a
fake native adapter. This does not establish real Incus aggregate export, public
artifact delivery, archive import, OS boot or SSH acceptance. Those remain planned
or unverified; the preceding native component tests prove only their own scope.

The existing Linux `TestRealIncusSnapshotAggregateE2E` now exercises the exporter
through the routed catalog and actual native producers after Base removal, then
reverifies all exported bytes after source mutation/deletion. Its existing GHA
gate runs this check. Dedicated WSL Incus 6.0.5/Btrfs acceptance passed in
314.12s, including confirmed temporary-capture cleanup before bundle return and
whole-bundle verification after source deletion. Existing snapshot restore and
fresh-generation/authorization-reset checks in the same fixture also passed;
that restore uses the saved snapshot, not the exported bundle. All owned fixture
resources and its recovery directory were cleaned. No export archive is retained.

The native run skipped shared source-image deletion and the optional public
snapshot/Workspace CLI paths because this invocation preserved the shared image
and supplied no CLI binary. The existing GHA aggregate gate supplies the CLI.
Actual SSH handshake, live OCI consistency and public bundle import remain
unverified or unimplemented. Full local CI (Go tests/vet, 27 notification tests)
and documentation checks passed on `bbcf7ea`; the first canonical integration
fixture failed on duplicate native names, corrected with per-capture identities.
No product ownership check or timeout was relaxed.
