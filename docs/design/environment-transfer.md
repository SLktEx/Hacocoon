# Environment transfer

Status: **partial overall**. Linux public export/import and Windows-file projected import are implemented and verified through the installed controller, fresh pinned SSH and retained-data recreation. Live OCI runtime consistency, Git reconnection acceptance and whole-installation evacuation remain incomplete.
See [Linux import command](#linux-import-command) for current usage and limits.

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

Public import creates new managed resources through canonical lifecycle
ownership, preserving Workspace Git state and retained OCI data. It must not
replace an existing Environment or restore old approval, connection or management
authority. Host credentials and control sockets are outside Environment export.
An archive checksum detects changed bytes; it is not authorization to apply its
configuration. Imported owner labels are source metadata, not a newly issued lease.

The dedicated Incus 6.0.5 CLI has no instance-import config/device override flags,
although the current upstream documentation describes them. Do not implement
import by assuming newer flags or by starting an instance before replacing its
old configuration. The implemented path imports an owned temporary image and creates an instance with
explicit current configuration. It validates rootfs input and reconstructs security
before startup; it does not activate an imported instance with source configuration.

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
template or aggregate-import acceptance. The original component test did not cover canonical
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
below. Public CLI wiring is implemented; acceptance remains pending.

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
the trusted controller stream. Import usage is documented below. The export source must be stopped:

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

Status: **implemented internally**; public importer acceptance remains pending.
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
on attachment or live OCI daemon was tested. Public import acceptance remains pending.

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

Status: **implemented** for public Linux export; aggregate import acceptance remains pending.
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
Public aggregate import acceptance remains pending; offline routing and native activation are implemented.

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
reconnection remains planned; native rootfs import and aggregate activation are implemented. At 634590d,
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

## Native bundle import composition

Status: **implemented internally; native bundle activation verified**. Import verifies the whole
bundle before mutation, imports one Workspace or a collection, imports OCI with a
durable association to that new Workspace, then creates/starts a new Env through
canonical lifecycle. The default destination is SOURCE-imported; an existing name
is refused. Public CLI/controller upload is connected; see the Linux import command below.

Version 2 preserves repository names and GitHub routing metadata without granting
authentication or approval. Source Host file URLs import offline. Version 1 imports
component labels offline because it has no routing descriptors. Guest Git data and
the original bundle are unchanged. Up to eight Workspace members are supported;
larger bundles fail before native mutation. Long repository names retain their
names while their private native IDs derive from fresh member owners.

Uncertain native creation keeps the existing receipts. Failed Env creation cleans
only published, unleased new data through existing ownership APIs; unresolved OCI
cleanup keeps its Workspace. Startup failure retains the Env/data. No schema change,
automatic backup, Base component or import recovery catalog is introduced. See
[ADR 0057](../adr/0057-native-bundle-import.md). Public import, reconnection, an SSH
handshake and live OCI consistency remain unfinished.

At 6360a23, the dedicated Incus/Btrfs aggregate passed in 558.35s, including
independent import of rootfs, both Git Workspaces and OCI after source deletion,
Env startup, generation checks, retained data after Env deletion and owned cleanup.
The local run omitted the CLI binary and skipped public CLI checks and shared-image
deletion. Public import, SSH handshake and live OCI consistency remain unverified.

## Management import transport

Status: **implemented for the Linux management endpoint**. The typed
`environment.import` stream accepts an optional destination name and bounded 64 KiB
data frames. Its explicit end record carries the byte count and SHA-256. The existing
importer stages and validates all input before native mutation. No client-selected
Host path, owner, OCI kind or budget is accepted. The current 64 GiB payload budget
plus bounded envelope overhead is shared with export.

Disconnect or additional input after upload cancels activation. The response returns
the existing import result, including retained-resource names on failure. Success
requires matching transfer count/digest, a running destination and terminal EOF.
The operation has a 30-minute deadline, and upload reads/writes have 30-second idle
deadlines. A caller-owned input reader must support finishing or cancellation.
No automatic retry or backup is added. The shipped Linux controller registers this
API only on its management endpoint, never guest or notification endpoints.

## Linux import command

Status: **partial**; CLI and shipped controller wiring are implemented. Dedicated
execution at b7297a3 verified shipped-CLI/fixture-controller native import, startup,
data retention and owned cleanup. Installed-controller/desktop import was initially
unverified and subsequently passed at 684e411; overall aggregate completion is recorded separately. Run from the Linux client that can read the bundle:

```bash
haco env import dev.haco
haco env import dev.haco new-dev
haco env import --json dev.haco new-dev
```

Only the file is required. The default destination uses the saved source name with
`-imported`; an existing Env or unresolved lease is refused. Import creates new
Workspace/OCI copies and a fresh Env, verifies current ownership/security and starts
it. The saved file and any existing Env/data are untouched. No Base material or
pre-import backup is required, and old approval/connection authority is not restored.

The client opens a read-only regular file, refuses final symlinks and special files,
inspects the bundle and uploads bytes. The controller validates the complete input
again before mutation. Input path replacement cannot change the open descriptor;
concurrent content changes cannot bypass the controller's complete bundle check.
The CLI's import operation has a 30-minute deadline. The shared payload limit is
64 GiB; there is no required size or storage argument.

A failed operation exits nonzero. `--json` also returns any retained-resource names;
plain output reports them on stderr. Inspect those resources before retrying because
a disconnect can leave an operation whose final result was not delivered. There is
no automatic retry. File routes and legacy bundles import offline; reconnection is
still planned. Linux/WSL is supported; native Windows file input is unsupported, and
files must be available to the client inside trusted `haco-host` when running there.
SSH handshake and live OCI daemon consistency remain separate acceptance items.

The first complete public-import aggregate at b7297a3 passed export, native import
and restore, then failed during public copy at the fixture's 12-minute deadline
(720.07s). This is an overall FAIL, not a successful aggregate or SKIP. Its ownership
catalog and saved data remain at `/var/lib/haco-snapshot-aggregate-1920048809` for
explicit cleanup; shared data was not selected for deletion. The expanded test
sequence now has a 20-minute fixture budget and a 25-minute GHA test-process budget.
Product timeouts and isolation are unchanged. At b7297a3, GHA's real Incus/Btrfs
[aggregate step](https://github.com/SLktEx/Hacocoon/actions/runs/34455660292/job/102801320149)
succeeded with the shipped import CLI and all aggregate assertions. This provides
independent acceptance while preserving the local failure record. The extended
local-budget variant has compiled but has not been rerun locally; all four b7297a3 workflows passed. The follow-up fixture-budget commit requires
its own latest-head CI result.

## Installed controller and SSH acceptance

The Incus aggregate includes a shipped-controller subtest with an empty private
catalog. It verifies native rootfs/Git/OCI, fresh generation, managed SSH key reset,
Env deletion retaining data and canonical owned cleanup. Diagnostics are kept
separately from the aggregate's strictly checked flat receipt directory.

| Commit | Actual result |
|---|---|
| a58d553 | Controller subtest PASS in 20.35s; aggregate FAIL in 89.56s because its final receipt check rejected the nested diagnostics. |
| 6d5e027 | SSH preparation FAIL before handshake; aggregate FAIL in 81.43s. Later cleanup correctly refused retained aggregate-owned resources. |
| e598270 | Confirmed sshd absent and failure in SSH provisioning; aggregate FAIL in 86.49s. |
| 0cc27a5 | Windows transfer FAIL at seed-repository (exit 127); VS Code PASS. The fixture now prepares Git in trusted Host and source Env through normal package installation. The rerun is pending. |

The bare fixture has no installed package-egress service. SSH continuation now
uses the existing Windows installed-product gate. It creates a separate managed
Workspace/OCI source and uses the existing narrowly scoped package Policy for
source SSH preparation. Windows OpenSSH creates unpushed/uncommitted/untracked
work; the source is stopped, exported, and its Env deleted before import through
the trusted Host client and installed controller. The imported Env receives no
source package grant: sshd comes from saved rootfs. A fresh controller-provided
host-key pin protects the real Windows SSH session. The gate checks Git, rootfs
and OCI markers, saves more work, deletes the Env, reattaches retained data to a
new Env, and explicitly deletes only its own test data through public commands.

This installed gate **passed at 684e411** (see the result below). Standalone native
controller checks remain required. The original external-path Windows SSH fixture
keeps its scope. Transfer failures are recorded while independent desktop probes
continue; neither gate is silently skipped to make CI pass. The bundle and raw
local test repository remain inside trusted Host for inspection. Windows-native
bundle file delivery and live Docker/containerd consistency remain unverified.
No product command, backend, Base component, backup or schema is added.

At 7517c27, transfer failed installing Git over Windows SSH (exit 100).
At 684e411, [Windows attempt 1](https://github.com/SLktEx/Hacocoon/actions/runs/34471376143/attempts/1)
passed Git installation, VS Code, export, source Env deletion, independent import,
fresh pinned Windows SSH, resumed work, retained Workspace/OCI recreation and
owned public cleanup. The bundle and raw fixture repository remain at
`/tmp/haco-transfer-4844a07f73644223` inside the trusted Host. OCI assertions use
synthetic persisted markers, not a running containerd/Docker workload.
All applicable native Incus/Btrfs and normal-test jobs passed at that commit.

The Windows job overall **failed** on the independent pending-approval probe:
`prepare-python-prerequisite-setup-start-internal`, exit 1, cleanup_failed=false.
Earlier project setup and subsequent preview/doctor probes passed; the cause is
unresolved. Attempt 2 failed at the same phase; transfer and independent desktop
checks passed again. A failure-only, read-only query through the existing pinned
SSH connection now reports only an allowlisted DNS service Result. This diagnostic
does not retry setup, restart services or change the failure result. Live OCI consistency and native Windows
bundle delivery remain unverified.

## Windows bundle file through existing drive projection

Status: **implemented; installed GHA acceptance passed at c4449e1** in [Windows run 34482712957](https://github.com/SLktEx/Hacocoon/actions/runs/34482712957). The
Windows gate copies the exported Linux bundle to a new Windows temporary file
through the trusted Host's existing drive projection. Exclusive file creation
refuses an existing target, and Windows checks its byte count and SHA-256 against
the export receipt. The source Env is then deleted and the ordinary Linux import
client reads that projected Windows file. Windows checks the file's digest again
after imported work and retained-data recreation. The Windows file is retained
outside the SSH fixture's cleanup directory for inspection.

This uses the existing file and management interfaces. It is not a native Windows
haco executable or direct export publication on DrvFS: export still first completes
on a supported Linux filesystem. The intended manual route is to copy that finished
bundle to a Windows folder, then use its projected path with the Linux client:

```bash
haco env import /mnt/c/Users/USER/Backups/dev.haco dev-imported
```

Native Windows export/import commands, automatic copying and whole-WSL evacuation
remain separate work. The local copy regression proves exclusive target handling;
the installed GHA gate passed this drive-projection route, including source Env deletion, imported work over Windows SSH, retained-data recreation and final bundle immutability. This does not establish a native Windows CLI, direct DrvFS export or another WSL installation.

## Live OCI transfer acceptance

Status: **implemented fixture; native acceptance passed at 6974272**. The existing aggregate
can opt into the same pinned containerd/nerdctl assets used by Store acceptance.
Its newly owned source executes an offline image, writes and syncs a file in a
named container's writable filesystem, exits that container, stops containerd and
then stops the source Env before ordinary aggregate export.

After source deletion, both the canonical importer and the shipped-controller
import check the saved image ID, verify there are no running tasks, explicitly
start the retained container and require its previously written bytes. No source
registry, image pull, old Base filesystem or task migration is involved. The
existing bundle immutability, ownership, fresh identity and data cleanup assertions
remain required. The fixture's native setup is not ordinary installed Base/runtime
installation acceptance. Docker, BuildKit cache and arbitrary application/database
consistency remain unverified; this slice covers the pinned containerd version,
native snapshotter and stopped container data only.

At ba4dbcd, native OCI acceptance FAILED during source runtime preparation before export. The fixture now identifies the fixed failed phase and exit code without raw subprocess output. Ownership recovery records remain; no transfer acceptance is claimed.

The offline source fixture explicitly configures the containerd transfer service for linux/amd64 native unpack. Its default unpack selection does not cover native; this is source preparation only. Import still replaces that configuration with current Hacocoon settings before starting the restored Environment. At 8103e3f, direct image import still failed before export; explicit CLI platform alone was insufficient. Native acceptance passed at 6974272 in [run 34501951826](https://github.com/SLktEx/Hacocoon/actions/runs/34501951826): aggregate 103.36s and shipped-controller import 22.00s, including source Env deletion and resumed containerd writable data. All applicable CI, including Windows, passed; the optional authenticated-private-registry job was skipped. Earlier failed attempts remain failures. No Docker, BuildKit/cache or arbitrary application consistency acceptance is claimed.

## Evacuation inventory

G2 is **partial**: `tools/evacuation_inventory.py` lists native Incus projects,
pools, instances, custom volumes and saved snapshots through read-only queries.
Run it from the repository on the Physical Host with existing Incus administration
access; it is an occasional recovery tool, not a new daily `haco` command:

```bash
umask 077
python3 tools/evacuation_inventory.py > inventory.json
```

The JSON contains resource names and types, not config bodies or credentials. Instance disk bindings include the pool, mount path and simple volume or Host-path reference; references are not opened or followed. Arbitrary URI sources are withheld with an explicit review marker. Every binding still requires ownership and external-data review. A malformed device leaves its binding unknown and the rest of the inventory available.
It preserves failed query labels and other successful results. Exit status 1 and
`native_queries_complete: false` mean at least one native query was incomplete.
Project views may refer to shared resources; rows do not establish distinct
ownership. The report grants no deletion or restore authority. Collection is bounded to 256 queries and five minutes between queries (each query has a 30-second deadline); reaching a bound preserves collected rows and reports incomplete inventory.

`backup_complete` is always false. The explicit unreviewed list still requires
catalog associations, controller/Policy settings, protected trusted Host data,
manual/unregistered files, external pools/VHDs and Windows references, readability,
consistent capture and restoration comparison. Neither all-file enumeration nor
export is implemented here. A successful native inventory is not whole-WSL
coverage. Keep the old WSL and data; no snapshot/create/delete operation is used.
The report itself must later be saved outside the storage being replaced.

A dedicated WSL Incus read passed: 2 project views, 1 pool, 13 instance rows and
55 volume rows, with no query errors. The private report remains at
`/var/tmp/haco-evacuation-inventory-tb_t97dj/inventory.json` inside that WSL; this is
not an external backup or a snapshot-deletion-failure evacuation test.

The extended dedicated WSL read also passed with 13 instance rows and 26 disk bindings, with no query errors. Its private report is /var/tmp/haco-evacuation-inventory-hp1r9_bs/inventory.json. Source references were only recorded, never opened; no external backup or ownership confirmation is implied.

The inventory now includes pool backing-source references and volume `content_type` alongside instance disk bindings. Paths are references for operator review: the helper does not open them, discover underlying VHDs, or infer ownership. A block volume must not be treated as a filesystem tree. URI-shaped sources are withheld to avoid publishing embedded credentials; malformed pool sources retain other inventory and mark the report incomplete. Eleven focused tests pass. The dedicated WSL Incus check also passed with no query errors: one Btrfs backing-path reference and filesystem volume content types were reported. Its private report remains at `/var/tmp/haco-evacuation-inventory-zcdndzfm/inventory.json`. External/block storage is covered by metadata tests only; no real block-volume evacuation is claimed.

Optionally add `--catalog /var/lib/hacocoon/state/environments.json` (substitute
the actual controller root). This Linux-only reader opens the existing regular
file without following a final symlink, refuses special/oversized/changing files,
and never invokes catalog migration or writes a lock/catalog file. Schema 13 is
currently supported; other schemas produce an explicit incomplete projection.
The allowlisted projection lists persistent-resource, Base-asset, Workspace-lease
and snapshot-component native references, including owner/generation evidence.
It does not validate those owners against Incus or grant restore/deletion authority.
`projection_complete` means only that these selected fields were read; repository
catalogs, pending operations, source paths, configuration/Policy/credentials and
manual data still require review. Unknown or malformed rows preserve other results
and cause exit status 1. Native queries and catalog projection have separate results;
`backup_complete` remains false. The input digest identifies the observed bytes,
not authenticity or an authorized migration. The file is never used to configure an Env.

Validation: 15 tests passed on Linux (Windows: 13 passed, 2 Linux-only tests skipped). Reading the existing dedicated WSL catalog failed explicitly because it is schema 4, with none of the four projected sections; it was not migrated. The failed observation receipt remains at `/var/tmp/haco-catalog-inventory-iuzdd46h/catalog.json`. A subsequent read of the retained native Incus aggregate catalog `/var/lib/haco-snapshot-aggregate-1920048809/state.json` passed: 9 records (3 snapshots, 3 persistent resources, 1 Base asset and 2 Workspace leases), with no projection errors. Its private receipt is `/var/tmp/haco-catalog-inventory-s5o_tdw4/catalog.json`. This is a real schema-13 reference observation, not native ownership comparison or full-data capture. The schema-4 default catalog was empty; no legacy migration was attempted.

Use `--repositories /var/lib/hacocoon/state/repositories` to include the separate repository records (adjust the controller root). The Linux reader pins the directory, does not recurse, and projects native references for each repo/work record and collection member. It omits remote URLs and credentials. Child symlinks, filename/record mismatches, unknown entries, oversized records and malformed members leave explicit gaps while retaining other results. The directory may change during observation; this is not an atomic installation snapshot or a complete backup. Whole-installation capture still needs quiescence and data comparison. The combined inventory regressions now include 17 cases; production repository-directory acceptance remains unverified.

Unreviewed repository entries include their directory-relative filenames alongside the error index, without opening their contents. This lets the operator locate manual files, links and malformed records; treat the inventory itself as private metadata. A read of the retained native aggregate directory extracted 3 repository files and 9 references, but exited with failure because 4 other entries still required review. Receipt: `/var/tmp/haco-repository-inventory-icbsgg5s/repositories.json`. This partial result is not complete repository-directory acceptance.

## Readable files when snapshot operations are unavailable

G2 file evacuation remains **partial**. Incus 6.0.5's Btrfs
[BackupVolume implementation](https://github.com/lxc/incus/blob/v6.0.5/internal/server/storage/drivers/driver_btrfs_volumes.go)
creates a temporary read-only snapshot even for non-optimized filesystem-volume
archives. `--volume-only` does not remove that internal dependency. Normal G1
export continues to use Incus backups; it is not the no-snapshot evacuation path.

The opt-in `TestRealIncusReadableDataEvacuationE2E` adds a direct GNU tar round trip
between two newly owned, unattached Btrfs custom volumes. It reuses the native
volume-transfer fixture and checks Git commits/status/untracked files, bytes,
hardlinks, symlinks, modes, numeric UID/GID, a user xattr, archive immutability and
independence after source deletion. Its archives stay in the fixture's private
directory outside both pools. No Incus export or snapshot command is used for
capture. Existing application data and pools are never selected.

The dedicated WSL Incus/Btrfs run passed in 20.59s. Both owned pools were cleaned; archives and ownership plan remain at /var/lib/haco-volume-transfer-2051477010, outside both pools but inside WSL. This verifies a quiescent file-copy primitive only.
It does not simulate a failed snapshot deletion, enumerate every installation
file, transfer a live OCI daemon, safely import arbitrary untrusted tar files,
encrypt trusted credentials, save the whole installation outside WSL or restore a new WSL. These remain
required before claiming whole-installation evacuation. An inaccessible or
changing source must not be reported as completely saved.

At a16f3b1, all applicable CI passed (the optional private-registry job was skipped).
The native GHA test passed in 0.84s in [run 34498433003](https://github.com/SLktEx/Hacocoon/actions/runs/34498433003).
A subsequent manual check exclusively copied the two synthetic archives to a new
Windows Temp directory, `C:/Users/gddro/AppData/Local/Temp/haco-readable-evacuation-egpq6g7c`.
The source before/after and destination SHA-256 checks passed, followed by independent
Windows byte-count and hash checks: `work.tar` is 112640 bytes and `oci.tar` is 10240 bytes.
The first Windows result-formatting attempt failed under constrained language mode;
plain-output verification then passed. `receipt.json` remains with the archives.
This proves delivery of these synthetic archives outside WSL, not whole-installation
backup, protected credential delivery or restoration in another WSL.

The additional `TestRealIncusSavedReadableDataEvacuationE2E` case prepares an owned native volume snapshot before capture, then removes a marker from the live volume. Direct tar capture reads the existing snapshot tree and restores that snapshot-only marker into a fresh owned volume. The ordinary live-volume case remains separate. Preparation creates a snapshot; capture does not create/export/delete one. This is not a simulation of snapshot deletion failure or proof of all saved rootfs/Workspace/OCI associations. The dedicated Incus/Btrfs run passed in 24.52s. Both owned pools were cleaned; archives and ownership plan remain at `/var/lib/haco-volume-transfer-2483709670`, outside both pools but inside WSL. The source snapshot was unchanged during capture. This does not prove whole-installation evacuation or new-WSL restoration.

At b8ef557, the native GHA gate passed direct readable-file evacuation (0.89s) and saved-only evacuation (1.87s). Its Windows SSH gate failed at the five-minute native-client timeout. The branch now includes the separately validated SSH progress diagnostic from main; the latest head must pass its own CI. This does not reclassify the earlier Windows failure or prove its cause.

## Encrypted readable-data transport

Whole-installation evacuation remains **partial**. Use existing GNU tar and
[age](https://github.com/FiloSottile/age) for reviewed, quiescent file trees rather
than introducing a Hacocoon encryption format. The private decryption identity
stays on trusted storage; encryption uses only its public recipient. The native
acceptance script `tools/test_encrypted_evacuation.py` uses synthetic credentials,
checks the producer and encryptor exit statuses, and writes no plaintext archive
to the external destination. It checks decryption and private file modes before
extracting only its own fixture, plus wrong-key, ciphertext-tamper and truncation
refusal. A failed or partial decryption must never be piped directly into restore.

For an already reviewed and stopped source tree, set SOURCE to that directory,
DEST to a new archive outside the WSL/pool being replaced, and RECIPIENT to your
age public recipient. Keep the matching private identity independently accessible:

```bash
umask 077
set -o pipefail
set -o noclobber
env -u TAR_OPTIONS tar --one-file-system --acls --xattrs --numeric-owner --sparse -C "$SOURCE" -cpf - . |
  age --recipient "$RECIPIENT" > "$DEST"
```

Check the whole pipeline's exit status; an output file alone is not completion.
Mounts skipped by `--one-file-system` require separate reviewed capture. Preserve
partial output for inspection and do not delete the source. Decrypt completely
into trusted private staging and verify success before considering extraction;
this is not a safe importer for arbitrary untrusted tar files. Actual credentials,
whole-installation coverage, key recovery after WSL removal and encrypted new-WSL restoration
remain unverified. The native script is opt-in with
`HACO_E2E_ENCRYPTED_EVACUATION=1`; `HACO_E2E_ENCRYPTED_OUTPUT_ROOT` can select an
existing external destination parent for its new synthetic test directory.

Dedicated WSL acceptance passed in 1.03s with age 1.2.1 (distribution package `age_1.2.1-1build1_amd64.deb`). Direct package installation failed because dpkg had an interrupted libc6 configuration; the package was then downloaded through apt and extracted into a private tool directory without changing system package state. Ciphertext and receipt remain at the new Windows output directory; Windows independently verified 10440 bytes and SHA-256 `bf25c5334464947c0ea0c8645ecc535195cd930dad98efd18550243323eb5804`. Synthetic source, restored data and test identities remain inside WSL at the private WSL test directory. This does not verify identity recovery after deleting WSL.

At 55be427 the existing native GHA [job](https://github.com/SLktEx/Hacocoon/actions/runs/34508162748/job/102975354767)
passed, including the real tar/age test in 0.029s. This result belongs to that
commit; the latest rebased head requires its own CI.

## Fresh WSL data restoration acceptance

Status: **partial G3**, separate from encrypted identity recovery. A fresh Ubuntu
26.04 WSL was imported under a new name using the official cached image after its
SHA-256 matched current Microsoft distribution metadata. Existing WSLs were retained.
This uses the standard [WSL import operation](https://learn.microsoft.com/en-us/windows/wsl/use-custom-distro),
not an old WSL filesystem or Incus database restored wholesale.

On Incus 6.0.5-8, synthetic Workspace/OCI tar archives previously evacuated to
Windows were restored into fresh, explicitly owned custom volumes in a new
1 GiB Btrfs pool. Their expected SHA-256 digests were checked before extraction;
only these known fixture archives were used. GNU tar comparison and explicit
checks passed for bytes, numeric owners, permissions, hardlinks, symlinks and
user xattrs. The saved Git HEAD and untracked file survived, and a new local
commit resumed work in the destination. The check passed in 8.04s and left the
source archives unchanged. Native snapshot creation/deletion and positive absence
then passed for a newly created snapshot of the restored Workspace volume.
The destination volumes and ownership receipts remain available for inspection.

This does not prove installed Hacocoon import, Env authority/network/credential
reconfiguration, live OCI application state, saved-only data in a new WSL,
whole-installation coverage or WSL replacement. Encrypted private-identity transfer
has not run. Complete those checks and review restored data before selecting an
old WSL for removal; this partial result authorizes no old-data deletion.
