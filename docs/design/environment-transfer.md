# Environment transfer

Status: **partial** for Linux public export; public import remains **planned**. The native rootfs/volume
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
The version-1 envelope remains readable; new Linux exports use version 2 as described
below. Public import is not implemented.

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

Status: **implemented internally**; Linux public delivery is partial as described below.
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

## Linux export command

Status: **partial**. Repository implementation provides `haco env export` through
the trusted controller stream; import remains planned. The source must be stopped:

```bash
haco env stop dev
haco env export dev
haco env export dev /path/to/dev.haco
```

Only the source name is required; the default file is `dev.haco` in the client's
current directory. Optional `--json` returns the path and byte/digest receipt.
The destination stays on the client and is never sent as a controller filesystem
path. Existing files are refused, including a name created concurrently. No extra
snapshot command, source deletion or automatic restore backup is required.

The management-only `environment.export` stream accepts only the source name.
It exports a canonical stopped capture using the controller's private
`$HACO_ROOT/transfers` directory, with a 64 GiB aggregate payload budget plus bounded
envelope overhead. Cancellation/disconnection cancels capture while canonical
cleanup retains uncertain ownership. The endpoint is not registered on guest Git
or read-only notification sockets.

Bounded canonical JSON frames carry at most 64 KiB of data each. The client requires
an explicit terminal count/SHA-256 receipt, successful cleanup and EOF; early EOF,
duplicate fields, extra frames, failed cleanup or a mismatched digest are failures.
The Linux CLI independently verifies the received envelope and source label in an
anonymous file, syncs it, and links the live inode into a pinned destination
folder without replacing an existing name. There is no named partial output or
pathname-based cleanup. This uses the documented [O_TMPFILE publication mechanism](https://man7.org/linux/man-pages/man2/open.2.html).

Client output currently requires Linux filesystem support for anonymous files
(e.g. ext4/Btrfs). Native Windows file publication and Windows-mounted output
acceptance remain unimplemented/unverified; there is no silent filesystem fallback.
A `haco` running inside trusted `haco-host` writes in that client's filesystem,
not implicitly on the Windows desktop. Public bundle import, fresh imported
authority, boot/SSH after import and complete G1 acceptance remain planned.

Unix stream and Linux filesystem/CLI race tests passed. An initial CLI regression
fixture failed to compile because its socket mode argument was missing; the fixture
was corrected. The existing native aggregate E2E now calls the shipped export CLI
when its CLI binary is supplied. The earlier 314.12s native result proves the
internal producer only; the public GHA result is recorded below.

The first dedicated run of the shipped public CLI passed export and source-Env
removal, then public snapshot create/restore, but failed at 480.07s when the
fixture's original eight-minute deadline killed the later copy command. This is
a failed full gate, not a successful gate or SKIP. Its exact catalog and retained
archive remain under `/var/lib/haco-snapshot-aggregate-2545909325`; the two test Environments were subsequently removed through canonical deletion
after exact generation verification. Workspace/OCI/snapshots and the failed-run
catalog remain retained for explicit cleanup. The aggregate fixture
now has twelve minutes for the added full-archive delivery/verification work, and
the existing CI invocation has fifteen minutes for its group of native tests.
Production deadlines and isolation are unchanged; corrected native acceptance is
still pending. Full local Go/vet/docs/notification CI passed on `081beda`.

The corrected dedicated run also failed, at 720.06s, during the final public
Workspace cleanup. Before that deadline, export, source deletion, public
snapshot restore and copy, same-name generation renewal, saved-copy independence,
managed SSH-key reset and native child snapshot/backup deletion refusal passed.
The failed fixture remains at `/var/lib/haco-snapshot-aggregate-462967548`;
remaining cleanup is not claimed successful. No further timeout increase is made.
The equivalent [GHA aggregate gate](https://github.com/SLktEx/Hacocoon/actions/runs/34430493864/job/102724802406)
passed in 47.06s at `3d0dd9a`, including shipped export, snapshot/restore/copy,
Workspace deletion and owned cleanup. All four applicable workflows passed. This substitutes Linux Incus/Btrfs coverage,
not a successful local WSL gate or an actual restored SSH handshake.

Postcheck found no Environment or Workspace lease in either failed fixture. The
original nine instances, protected sentinel SHA-256 and registration mode/link count
were unchanged. All four failed-fixture snapshots were then verified component by component and
deleted through the canonical API. Four retained OCI Stores, their Workspace
records and two complete export archives remain for explicit cleanup.

## Verified component delivery for import

Status: **implemented internally**; the public importer remains planned.
`Staged.ComponentReader(role)` exposes a seekable, read-only view of one native
archive. Its offsets come from the same bounded parser that verifies every
component and the complete envelope; no component view is published on partial
validation. Each reader has its own cursor and cannot read adjacent component
bytes, the manifest or an outer file handle. Closing the staged bundle invalidates
its readers. No archive is extracted into a Host directory.

This is the input boundary for the future Incus image/volume import adapter, not
permission to restore source configuration. Inner archive validation, exact new
native ownership, canonical Workspace/OCI registration, fresh Environment creation
and current security setup are still required. No CLI argument or catalog schema
is added by this boundary. Linux real-filesystem component tests and the existing
transfer suite passed with the race detector; vet passed. No native import ran.

## Native OCI volume import

Status: **implemented internally**. The persistent-resource service's import method
uses the existing new-owner `creating`/verify/`ready` transition. The Linux Incus
adapter prepares a private anonymous archive with fresh native metadata before
calling ordinary `incus storage volume import`; there is no post-import ownership
repair or new recovery catalog. See [ADR 0051](../adr/0051-native-import-ownership.md).

The initial input is an uncompressed, non-optimized Btrfs filesystem-volume archive
without child snapshots. Its index is limited to 64 KiB, paths to 4096 bytes and
entries to one million, within the controller's archive budget. Source authority
is discarded; validated idmap bookkeeping remains with numeric file IDs. Existing
volumes, unsafe paths/links, duplicate metadata and incomplete archives are refused.
Unsupported formats fail explicitly. Native extraction remains Incus's responsibility.

The dedicated Incus/Btrfs gate passed in 0.56s using two fresh pools and a real
catalog: new owner/config, data, hardlinks, symlinks, mode, numeric UID/GID and idmap,
duplicate refusal, independent mutation, source deletion and canonical owned cleanup.
Both pools were removed after emptiness/marker checks. The original archive and
plan remain at `/var/lib/haco-owned-import-4138767719`. `HACO_E2E_INCUS_VOLUME_IMPORT=1`
runs the gate; it is also connected to the existing Incus GHA job.

Focused Store/import and native preparation race tests passed (1.052s/1.057s);
vet passed. The first fixture build failed on a quoted multiline string and was
corrected. No rootfs/Workspace aggregate import, Env activation, actual idmap shift
on attachment or live OCI daemon was tested. Public import remains planned.

The native runner boundary regression also covers owned/foreign target refusal,
malformed/truncated inventory, failed inventory queries, nonzero native exit and
lost native replies. It checks fresh metadata at the actual import invocation and
closes anonymous input after both success and failure. Focused race tests passed
in 2.359s. The initial PR head's existing Incus GHA job also passed the owned
volume-import step; this is not yet an all-green PR result.

The version-2 work below addresses the previously missing registration metadata: the
version-1 envelope carries ordered archives but no repository name, remote or
branch mapping. The current managed Workspace service needs that mapping; guest
Git configuration must not silently become trusted broker routing. This remains
part of public import implementation, not a claim that existing bundles are lost
or unreadable. Existing inspection/component access remains supported.

## Workspace routing in new exports

Status: **implemented** for public Linux export; aggregate import remains planned.
New public exports use envelope version 2 and include ordered Workspace
name/remote/branch records from protected saved bindings. The role ties each record
to exactly one native archive. Names are unique; existing Git validators reject
credentials and unsupported routing, with a 4096-byte remote bound and the existing
64 KiB manifest budget. Staged metadata copies cannot mutate verified state.

These fields are data, not transferred permission. No source owners, approvals,
credentials or native paths are added. Local `file:` remotes describe the source
Host only and must not authorize destination Host access. Missing legacy routing
remains empty. See [ADR 0052](../adr/0052-transfer-routing-metadata.md).

The command remains `haco env export <stopped-env> [file.haco]`. Existing version-1
bundles still support full inspection and component reads; there is no migration
or rewrite of saved data. Readers predating version 2 explicitly reject new
exports. Use an updated Hacocoon reader. Public import still needs fresh Workspace
registration, missing/local routing handling, rootfs import and Env activation.

Focused transfer, Router and Incus metadata tests passed (0.612s/0.089s/0.298s);
composition compiled with no selected tests. Documentation checks passed. The first
build failed because the Router forwarding method was missing; it was added with
a mixed-route regression before the passing run. Full CI and native acceptance
for version-2 export remain pending.

## Native Workspace registration

Status: **implemented internally** for a single Workspace with explicit GitHub
or offline routing. `RepositoryService.ImportWorkspace` reuses normal ownership reservation,
created/inspect/ready publication, and deliberately skips Git population. Native
volume import uses the same bounded Incus archive preparation as OCI import with
fresh Workspace config. Existing targets are refused before import. No clone,
checkout, remote request, guest hook or credential operation is run.

Source local-file URLs remain refused by this registration method. Explicit empty
routing and multi-Workspace composition are now supported internally as described
below; public metadata mapping and reconnection remain planned. Failure retains the exact incomplete record; public aggregate cleanup is
not implemented. See [ADR 0053](../adr/0053-workspace-native-import.md). Existing
catalog schema, normal clone/copy behavior and ready Workspace deletion are unchanged.

Focused service/native-boundary tests passed (0.427s/0.536s), including durable
ownership before import, no population, duplicate refusal and failure retention.
The existing native volume E2E is extended to register Workspace data after source
volume deletion and check commits, dirty/untracked files, preserved guest Git
config, independent management routing and owned cleanup. The dedicated Incus/Btrfs
run passed in 20.67s at `b7c7ec5`, including canonical Workspace registration/deletion
and both isolated pool deletions. The original archive and fixture plan remain at
`/var/lib/haco-owned-import-1590527782`. Postcheck confirmed the original nine
instances, sentinel checksum and registration mode/link count unchanged.

All local Go tests, vet, docs/workflow policy and 27 JavaScript tests passed on the
same source. Focused race tests passed for the Git service (1.568s) and native adapter (2.541s);
the complete local validation invocation exited successfully. This does not prove Env attachment/idmap shifting, boot,
SSH, a live OCI daemon, multi-Workspace import or the public aggregate command.

## Cleanup after native Workspace import failure

Status: **implemented internally** for completed, unpublished single-Workspace
creation. The failed import cleans up only when its returned and re-read registry
record are identical and `created`. The existing native deletion contract checks
owner, users and saved children and confirms absence before the registry is removed.
Cleanup has a bounded context independent of client cancellation. An import that
was cleaned up still returns failure; it never becomes a ready Workspace.

A `creating` result may have an unresolved native request, and a `ready` result may
have consumers after a publication error. Neither is automatically deleted. Changed
identity or uncertain cleanup retains/returns the exact receipt and reports
recovery-required. No automatic replay, hidden backup or new state is added. See
[ADR 0054](../adr/0054-completed-import-cleanup.md).

Focused race tests and vet passed (2.072s) for cleanup completion/failure, changed
owner, publication, cancellation and unknown creation. The existing real Incus
volume-import gate now injects post-create verification failure, cleanup failure
and lost native replies. Dedicated Incus/Btrfs acceptance passed in 21.65s at
2917714; full local Go/vet/docs/workflow-policy/JS checks and focused race (1.928s)
also passed. Archive/receipts remain at /var/lib/haco-owned-import-490401733.
Unknown-creation and cleanup-failure receipts remain; test-only teardown removes
their native fixtures after known completed creation. This does not prove
production uncertain cleanup succeeded. Postcheck confirmed the original nine
instances, sentinel checksum and registration attributes unchanged. Unresolved creation and published aggregate cleanup are still
explicit remaining work; this is not a general incomplete-Workspace repair API.

## Native multi-Workspace registration

Status: **implemented internally** for two to eight Workspace archives with explicit
GitHub or offline routing. Import shares normal collection creation: reserve all fresh member
identities in one record, import and durably record each completed native creation,
inspect each volume, then publish the whole collection. Git population is omitted.
Members have no separately resolvable records. Distinct native targets are required;
invalid input is rejected before reservation. Incus owns the actual volume imports.

Partial failure retains the complete collection receipt and completed/uncertain
member identities. It never publishes a partial Workspace, replays unknown imports,
or applies the single-volume cleanup helper to a collection. Explicit incomplete
collection cleanup remains needed before public import ships. Ready collections use
existing canonical lease exclusion and owned member deletion. No new catalog,
schema, state, CLI command or source permission is introduced. Existing records do
not require migration. This extends ADR 0053 using the existing collection model.

At b7dca44, all local Go/vet/docs/workflow-policy and 27 JavaScript tests passed;
focused race passed (2.782s). Dedicated real Incus/Btrfs acceptance passed (29.64s):
source-deleted archives, two independent native copies, retained Git/untracked data,
no separate member records and canonical collection deletion. Partial collection
failure retention is covered by service tests, not a native injected-failure run.
Public aggregate import, offline routing, rootfs and Env activation remain planned.

## Offline Workspace data

Status: **implemented internally**. Empty remote and branch register an offline
Workspace; partial routing is invalid and imported source file URLs remain refused.
No synthetic Host repository, approval or credential is created. Mixed collections
retain data for offline members while the Git broker binds only configured members.
Online bindings require an exact remote/branch match with the current Host source.
Snapshot Workspace copies with absent routing can now restore offline as well.

Existing catalog fields and source archives are retained unchanged. No schema or
CLI change is needed. Offline members do not keep unrelated same-name source
repositories alive; their native data remains subject to normal owned deletion.
See [ADR 0055](../adr/0055-offline-workspace-routing.md). Public metadata mapping,
reconnection, rootfs import and aggregate activation remain planned. At 634590d,
all local Go/vet/docs/workflow-policy and 27 JavaScript tests passed; focused race
passed for Git (11.451s) and Incus (2.386s). Dedicated real Incus/Btrfs mixed-
collection import and native attachment-metadata checks passed in 29.52s. Offline
snapshot copying and broker refusal are covered by component/service tests; real
offline snapshot restore, attached/running Env and live Git/OCI acceptance remain
unverified.

## Native rootfs image import

Status: **implemented internally**. Native Incus/Btrfs transport acceptance passed
(22.28s), including source removal and independent destination data after temporary
image deletion. This does not verify boot, SSH or public aggregate import. A bounded anonymous archive
preserves rootfs data while replacing image properties with a fresh import owner
and omitting image creation templates. The original archive stays unchanged.
Incus imports the unified container image; a synchronous consumer must create an
independent instance with current explicit configuration. The temporary image is
then removed using exact ownership and positive absence checks shared with export.
Unconfirmed creation or cleanup keeps the receipt; no Base or backup is added.

The pinned SDK's context-aware raw operation provides native upload and operation
waiting. Current local Unix daemon/project and bounded response checks remain.
Only uncompressed unified x86_64/aarch64 container images are supported initially.
See [ADR 0056](../adr/0056-native-rootfs-import.md) and canonical creation below.
Public aggregate import, SSH handshake and live OCI consistency remain unfinished.

## Archive to Environment

Status: **implemented internally; native activation verified**. The Workspace service's
CreateFromArchive uses canonical Env creation with caller-prepared Workspace/OCI
bindings. It skips current Host OCI defaults. The Incus adapter initializes an
independent instance from the owned temporary image, records ownership immediately,
then applies current sandbox configuration and renews guest SSH identity. Unknown
native init completion retains the lease; cleanup never removes attached data.
No CLI, catalog migration, Base filesystem or automatic backup is added. Full
bundle orchestration, public usage and an SSH handshake remain unverified.

The production BaseRouter forwards this native archive to Incus through the shared
receipt protocol. The existing aggregate E2E now exercises this router with an
exported bundle after prior Env deletion, followed by native startup, explicit
Workspace/OCI binding, generation/managed SSH reset and deletion retaining data.
Dedicated Incus/Btrfs execution at 47bf7a8 passed in 439.76s. The same run covered
existing capture/restore/export and native data cleanup. Public CLI checks were
skipped locally because no CLI binary was supplied; GHA supplies that binary.
Shared-image deletion was skipped. Public import, an SSH transport handshake and
live OCI runtime consistency are not established by this test.
