# Environment transfer

[日本語](environment-transfer.ja.md) | English

Status: **partial overall**. Linux export/import, installed-controller delivery,
Windows projected-file import and one managed cross-WSL fixture have acceptance.
Stopped containerd image identity and writable data also have real Incus/Btrfs acceptance;
resuming the saved container requires an explicit start, not running-process migration.
Whole-installation evacuation, imported authenticated Git and broader OCI/application
consistency remain incomplete. [Evidence and failed gates](../status/acceptance-evidence.md#transfer)
are separate from the contract below.

<a id="commands"></a>

## Export and import

Run in the Linux client domain that can read/write the selected file (normally
trusted `haco-host`). Stop application writers and the source Env first:

```bash
haco env stop dev
haco env export dev dev.haco
haco env import dev.haco recovered
haco open --client ssh recovered
```

Export defaults to `<source>.haco` in the client's current directory; import defaults
to `<source>-imported`. Both accept `--json`. Existing output files, destination
Envs and unresolved destination leases are refused. Export creates its stopped
capture internally; no separate snapshot command is required.

Import creates new managed Workspace/OCI data and a fresh running Env. The archive,
source Env and existing data are untouched. Base is provenance only; no Base
filesystem or pre-import backup is required. Old approvals, connections and
management authority are never restored. Set up fresh SSH and review Git routing/
permissions as described in the [Git guide](../guides/git-workflow.md).

Export publication needs Linux `openat2`/`O_TMPFILE` support, such as ext4/Btrfs.
An unsupported filesystem fails explicitly. Export from trusted Host does not
implicitly write to the Windows desktop. Copy the complete verified bundle to
Windows through the existing drive projection, verify its size/SHA-256 there, then
import with a readable projected path, for example:

```bash
haco env import /mnt/c/Users/USER/Backups/dev.haco recovered
```

Replace `USER` and the filename with your retained file. Direct DrvFS export
publication and a native Windows CLI are not established by projected-file acceptance.
Keep the original installation until [evacuation/restore comparison](../guides/data-evacuation.md)
has established the required data is independently recoverable.

Failure returns nonzero and retained resource names (JSON or stderr).
A lost reply can leave completed or uncertain work: inspect recorded identities
before retrying. No automatic replay, overwrite or catalog-edit recovery is provided.

## Native substrate and authority

Incus exports rootfs as a unified image and attached custom volumes separately.
An instance archive alone does not include Workspace/OCI volume contents.
Ordinary archives can cross pools; optimized archives require compatible drivers.

Incus 6.0.5 lacks newer instance-import configuration override flags. The current
adapter imports a temporary owned image, then creates an independent instance
using current explicit configuration and no inherited profiles. It validates data,
records ownership immediately after init and reconstructs network/source guards and
SSH identity before start. It never starts an instance with source management config.

Imported labels/checksums describe data; they do not authorize a lease, policy,
Host path, credential or configuration. The management-only controller endpoint
is absent from guest Git and notification sockets. No inner archive is extracted
into a caller-selected Host directory.

## Bundle and verified staging

`internal/environmenttransfer` uses a bounded USTAR envelope with canonical JSON.
Entries are `manifest.json`, `rootfs.tar`, `workspace.tar`, consecutive
`workspace-002.tar` through `workspace-253.tar`, and optional `oci.tar`.
Metadata is at most 64 KiB; envelope overhead at most 512 KiB; the public aggregate
payload budget is 64 GiB. This budget is validation, not a disk quota while Incus writes.

Version 2 exports preserve repository names and GitHub routing descriptors.
Version 1 remains readable and imports offline. Older readers reject version 2.
Source local-file routes import offline; missing routing cannot grant destination
Host access. Current aggregate import supports at most **eight** Workspace members;
larger valid envelopes fail before native mutation. Partial routing is invalid.
Online reconnect requires an exact current Host repository remote/branch match.

Verification rejects duplicate/unknown JSON fields, missing/extra/reordered roles,
incorrect sizes/hashes, overflow, links/extended headers, incomplete closing blocks
and trailing data. It completes before any consumer or native mutation.
The writer compares every archive with the protected complete snapshot inventory;
an untrusted manifest alone cannot prove source completeness. Legacy Base records
remain in the catalog but require no Base archive; unknown roles fail.

Linux staging pins a private controller directory, creates an anonymous file,
closes its writable descriptor and verifies the whole envelope before exposing
bounded read-only component readers. Components have independent cursors and cannot
read adjacent bytes. Closing the staged bundle invalidates readers. Path replacement
cannot replace the pinned bytes; privileged process-descriptor access is outside
this guarantee. There is no weaker filesystem fallback, permission repair or durable
staging-recovery catalog.

## Capture and transport lifetime

`ReadSnapshot` holds canonical Environment-then-Workspace locks through all native
component reads, revalidating source identity and ready state. Returning a snapshot
value is not a reservation. Consumers must finish before returning and cannot re-enter
lifecycle operations. Cancellation releases locks without deleting the saved source.

Custom-volume export verifies exact detached ownership before and after the native
operation. It compares native backup identities/times/flags before and after export:
new/changed leftovers or failed observation prevent success, because Incus can ignore
deferred backup-deletion errors. Existing native backups are never deleted by name.

Rootfs export/import uses the selected private Unix Incus remote/project, not an
implicit fallback. HTTPS/clustered remotes and split images are unsupported.
Bounded SDK metadata and context-aware image transfers reject redirects and verify
fingerprint/hash. Rootfs import currently accepts uncompressed unified x86_64/aarch64
container images, replaces image properties with a new import owner and omits creation
templates. Source archive bytes remain unchanged.

Before temporary publication, a private receipt records socket/project/random owner.
Native operation/fingerprint are durably appended before further checks. Only exactly
owned alias-free images can be removed, and positive absence precedes receipt deletion.
Uncertain publication/cleanup retains the receipt; source saved data is not removed.

Management streams use canonical frames of at most 64 KiB and a terminal count/SHA-256.
Export succeeds only after complete verification, successful cleanup and EOF. The client
syncs an anonymous file and links that live inode into a pinned destination without
overwriting. There is no named partial output or pathname-based cleanup.

Import opens a regular read-only file, refuses final symlinks/special files, uploads it
and revalidates the entire input in the controller. The operation deadline is 30 minutes,
with 30-second upload idle deadlines. Disconnect/additional input cancels activation.
A successful response requires matching bytes/digest, a running destination and EOF.

## Registration and failed cleanup

Import reserves fresh identities before native creation, records each completion,
verifies native metadata/idmap correspondence and publishes only the complete ready
Workspace/collection. It never runs source Git hooks, clone, checkout or credential
operations. Collections have one lease and no separately resolvable members.

OCI import binds a fresh resource to the new Workspace. Canonical Env creation uses
these explicit data bindings and skips current Host defaults. Native volume metadata
is reconstructed for the new owner; source configuration is not authority.

A completed unpublished single-Workspace import may be cleaned only when its
re-read receipt still matches. Native deletion checks ownership, consumers and saved
children under a cancellation-independent deadline. Unknown `creating` state,
published `ready` data, changed identity and partial collection creation remain retained.
An import with successful cleanup still reports its original failure.

Failed Env creation cleans only published, unleased new data through existing APIs;
uncertain OCI cleanup keeps its Workspace. Startup failure retains the Env/data.
No lease is released merely because cleanup was attempted.

## Design rationale and verification

[ADR 0049](../adr/0049-transfer-envelope-authority.md), [ADR 0052](../adr/0052-transfer-routing-metadata.md),
[ADR 0053](../adr/0053-workspace-native-import.md), [ADR 0054](../adr/0054-completed-import-cleanup.md),
[ADR 0056](../adr/0056-native-rootfs-import.md) and [ADR 0057](../adr/0057-native-bundle-import.md)
retain the boundary decisions and rejected shortcuts.

Repository tests cover malformed input, bounded reads, path replacement, missing
components, source locks, durable receipts, fresh authority and failed cleanup.
Native component tests, complete public flows and cross-WSL fixtures prove different
scopes; [acceptance evidence](../status/acceptance-evidence.md#transfer) retains their
failures and skips. A stopped containerd fixture is not arbitrary live-application
consistency, and a managed bundle is not a complete installation backup.

## Incus architecture names in rootfs archives

Implemented: rootfs import resolves architecture names through the pinned Incus
SDK, then retains the existing x86_64/aarch64 CPU restriction and writes the
canonical name to its private transport image. Incus aliases such as amd64 and
arm64 describe those same CPUs; unknown and other architectures still fail
closed. The source archive, ownership checks, template removal and resource
lifetime are unchanged.


See [native validation](../status/acceptance-evidence.md#development-branch-integration) for the scoped acceptance and remaining gaps.
