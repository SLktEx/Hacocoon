# Environment snapshots and restore

Status: **partial**. Internal capture, independent restore copies and exact-owned
cleanup are implemented. Publishing a restored runnable Environment and the
public snapshot/restore CLI remain planned. See [implementation status](../IMPLEMENTATION_STATUS.md).

## Incus owns storage and runtime operations

Hacocoon starts from Incus instance, custom-volume, copy, device and lifecycle
operations. The supported aggregate is a stopped Incus container and managed
Workspace/OCI volumes in one Incus Btrfs pool. Incus provides COW copies; Hacocoon
does not implement a storage engine or a generic fallback for other backends.
Other drivers are currently unsupported for this aggregate, not assumed to have
identical consistency semantics with merely different speed.

An Incus instance snapshot belongs to its source instance. Hacocoon therefore
uses independent instance and volume **copies**, excluding dependent snapshots,
so deleting the source Environment cannot delete saved data. See the Incus
[Btrfs driver](https://linuxcontainers.org/incus/docs/main/reference/storage_btrfs/)
and [instance backup guide](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/).

## Saved contents

A complete manifest owns:

- one independent stopped rootfs instance;
- every registered managed Workspace volume, including Git objects, unpushed
  commits, uncommitted and untracked files;
- the attached OCI Store, when present;
- source generation, Base provenance, exact resource ownership and mount layout.

New captures do **not** save a Base filesystem component. The rootfs is complete;
Base name/revision/fingerprint are provenance only. Neither planning, reading nor
copying this saved rootfs requires the original Base or image cache. Ordinary
Environment creation resolves its Base to an Incus image and does not automatically
create another retained Base instance.

Old snapshots may still own a Base component. Keep that record and its storage
until explicit snapshot deletion. New restore preparation skips that legacy
component, without modifying the source manifest. Legacy Base verification and
exact-owned cleanup remain for existing material; they are not a new retention
layer. See [ADR 0040](../adr/0040-incus-first-snapshots.md).

## Capture and ownership boundary

`internal/workspace` holds canonical Environment then Workspace locks and checks
the current active lease and fresh creation identity. `modules/runtime/incus`
enumerates the full device/volume inventory, verifies ownership, stopped state,
exclusive attachments and Btrfs placement. External-path Workspaces remain
unsupported here because guest stop does not exclude Host writers.

The catalog reserves planned exact identities before creation. Each successful
create is durably recorded before another fallible provider call. All components
must verify before the aggregate becomes ready. Partial results are never a
successful snapshot. Stopped-source reservations protect data consistency until
the incomplete capture is explicitly cleaned up; this is not a promise of full
runtime crash recovery.

Saved instances have no profiles or active non-root devices, no autostart and
fresh storage ownership. Workload config is explicitly cleared except required
Incus idmap bookkeeping. Host credentials, controller state, old network access,
management authority and approval records are not replayable snapshot settings.

## Restore preparation and future activation

The implemented flow is: select a ready snapshot, verify its required saved
components, reserve the current target identity and independent destination names,
copy rootfs/Workspace/OCI, record each completion, and verify the complete result.
Current Environment data remains untouched. No automatic pre-restore backup,
rollback snapshot or hidden equivalent is created. Save explicitly first if the
current state should be retained after a future explicit replacement.

Prepared copies are stopped/unattached and are not a runnable Environment. The
remaining activation must use the canonical lifecycle API, a new creation ID,
current network/security configuration and fresh connection credentials. It must
not adopt the old source's devices, grants or authentication. Prepare independent
data before switching; replace only the target data explicitly selected by the
user. Same-name recreation must not inherit the prior generation's approvals.

On failure, the current implementation reports the restore ID and retains exact
owned destinations for `CleanupSnapshotRestore`. Cleanup removes only verified
owned copies, positively checks absence and keeps records on ambiguous results.
Retry cleanup or start a fresh preparation after cleanup. There is no activation
rollback or automatic resume from every crash point. Future activation must not
introduce these as prerequisites for disposable Environments.

## Lifetime and ordinary recreation

Normal Env delete removes the runnable instance, not Workspace, retained OCI,
explicit persistent data or saved snapshots. Recreate checks those associations,
deletes the old runtime and creates a new one with current data and setup.
Unsaved rootfs/process/temp/manual changes can be lost. Snapshot restore instead
uses the saved point-in-time data. Saving rootfs makes that copy persistent even
though the executing Environment remains disposable.

## Catalog upgrade

Schema 10 adds the current target identity independently of saved data. Schema 8
restore records migrate on write by deriving it from their recorded `before`
source. Their existing backup manifest and snapshot references remain intact;
cleanup does not delete those snapshots. Existing schema 5–8 snapshot ownership
and schema 7–8 Base assets remain readable. No stored Base component is silently
removed and no schema is downgraded. Schema 9 belonged to an unpublished local
replacement prototype and is explicitly rejected; keep its original binary and
ownership file for scoped cleanup, rather than relabeling it as a released schema.
Older controllers reject the new schema. Stop the controller before changing
binaries; do not edit the schema number manually. There is no automatic data
removal or general-purpose migration/rollback framework.

## Package and interface responsibilities

- `modules/runtime/incus`: native copy/instance/volume/device operations and exact
  provider observations. Btrfs is an explicit supported precondition.
- `internal/workspace`: aggregate locks and ordered capture/preparation/cleanup.
- `internal/state`: durable data/generation ownership and atomic lifecycle guards.
- `internal/environment`: routing and qualification of Incus native references.
- `internal/core`: small manifests and domain identity types.

`SnapshotBackend` and `RestoreBackend` keep native storage mechanics testable and
out of orchestration. The catalog interface protects receipts and atomic checks.
These interfaces do not promise hypothetical future backend equivalence. Removed
production Base-retention callbacks are not replaced with another abstraction.
Legacy Base storage code remains only to read, verify and clean recorded material
and to retain its existing focused tests.

## Validation scope

Tests cover omitted members, source generation drift, foreign attachments/owners,
partial completion, deletion races, legacy migration, no new Base/backup and exact
cleanup. The opt-in real aggregate fixture exercises Incus/Btrfs with two Git
Workspaces, OCI file data and rootfs; it deletes its owned Base before capture and
optionally deletes the isolated source image. It checks staged bytes, independent
edits, unchanged current data, normal Env deletion and source-independent saved
data. Execution results belong in implementation status. File fixtures do not
establish live Docker/containerd database consistency or runnable restore.
