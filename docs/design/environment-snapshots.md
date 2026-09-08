# Environment snapshots and restore

Status: **partial**. Public capture/list/delete, independent restore copies and
canonical saved-rootfs creation are implemented. Public aggregate restore remains
planned. See [implementation status](../IMPLEMENTATION_STATUS.md).

## Daily snapshot commands

```bash
haco snapshot create dev
haco snapshot list dev
haco snapshot delete snap-0123456789abcdef0123456789abcdef
```

`create` requires a managed Workspace. It holds the normal lifecycle locks,
verifies the current generation, and stops a running Env through Incus before
copying. It restarts that same generation only after the complete save is ready;
an already stopped Env stays stopped. Expect interruption of running processes.
This saves filesystem data, not process state or live database consistency.

`list` without an Env shows all saved records, including incomplete captures and
saves whose source Env has been deleted. `create` and `list` support `--json`
before the positional argument. Results contain IDs/state and aggregate counts,
not private storage bindings or management configuration.

A failed capture stays stopped; a partial save keeps its ID and ownership record.
Inspect with `list`, then explicitly delete that ID to clean its owned components
before starting the source again. If saving completed but restart failed, the
command returns failure while retaining the ready ID. Use normal Env status/start
to inspect and resume it. A disconnected client is not proof that capture failed:
check `list` before retrying. No automatic backup or runtime rollback is created.

`delete` requires the full ID and affects only the saved copies. It preserves
current Env/Workspace/OCI data, checks active restore reservations, and retains
ownership records when cleanup cannot establish positive absence. This is the
explicit removal operation for complete or incomplete saved data.

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

Native instance-copy configuration uses the same clearing helper for capture,
staging and runnable copies. Incus's `volatile.last_state.ready` is reset to
`false`, never an empty string or the source's ready value: the
[Incus 6.0.5 validator](https://github.com/lxc/incus/blob/v6.0.5/internal/instance/config.go)
requires a boolean even for this volatile key. This does not preserve processes
or old authority, and changes no saved-data schema.

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

On failure, the service attempts bounded cleanup under the existing locks. It
returns failure even if cleanup succeeds, without retaining a recovery reservation.
If cleanup cannot confirm absence, the error reports the restore ID and preserves
exact destinations for `CleanupSnapshotRestore`. Both paths remove only verified
owned copies, positively check absence and keep records on ambiguous results.
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

Schema 10 added the current target identity independently of saved data.
Schema 11 retains that data and adds OCI-copy ownership receipts and temporary
saved-source reservations; schema 10 migrates on write without deleting anything. Schema 8
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

## Shared runtime configuration

The ordinary SandboxProvider post-init path is isolated in
`sandbox_configuration.go`: network/source guard, managed marker, resource limits,
Workspace/OCI attachments, start, anti-spoof checks, DNS and data-root setup stay in
one implementation. Production ordinary creation records its runtime before this
fallible phase using the canonical lifecycle receipt described in
[ADR 0002](../adr/0002-environment-lifecycle-ownership.md).

`snapshot_runtime.go` implements the internal native saved-rootfs creation path.
It copies the verified independent saved instance in its Btrfs pool without
resolving an original Base, image or default profile. The caller holds the saved
aggregate reservation and canonical creation lease, supplies newly registered
Workspace/OCI bindings and a fresh generation, and records the creation receipt
before configuration. On that newly owned runtime only, remove the copied `none`
device masks before adding current attachments; Incus preserves those names and
otherwise rejects duplicate device additions. Current network/source guards,
limits and data attachments use the ordinary path. Saved profiles/devices/authority-bearing config are not
replayed. Only Incus filesystem idmap bookkeeping survives the rootfs copy.

Before publication, remove Hacocoon-managed SSH authorized-key entries, preserve
user entries, regenerate installed SSH host keys and restart an installed SSH
service. Failure remains failure and the caller owns bounded exact-owned cleanup;
ambiguous creation keeps the reserved name/generation for inspection. No new
runtime recovery state machine or automatic backup is added. The saved source is
unchanged. This internal primitive does not adopt arbitrary existing instances.
Aggregate activation orchestration, target switching and public CLI remain planned.
Independent Workspace and OCI registration are implemented internally; the latter
uses the existing [Store catalog](persistent-oci-store.md#store-registration-from-a-snapshot).

## Registering restored Workspaces

Implemented internally: `RepositoryService.RestoreWorkspace` registers one or up
to eight independent normal managed Workspace volumes from saved components.
The service requires the shared snapshot catalog and holds a durable source
reservation while copying. Incus performs same-pool Btrfs copies;
the registry reserves fresh names/owners before copy, records each completion
before verification, and publishes only the complete collection. Source Env,
source volumes, Base and image cache are unnecessary. No Git network operation
or guest configuration rewrite runs during the copy. This is data preparation,
not runnable Environment activation or completed Git-broker acceptance.

New volume bindings include credential-free remote/branch provenance from the
trusted Workspace registry. Guest `.git/config` is never promoted to Host routing
or authorization policy. Old bindings omit these fields and remain readable,
verifiable and deletable without rewriting their saved bytes. Automatic Workspace
registration returns unsupported when this provenance is absent. Re-save from an
available original Environment to obtain it; a source-less legacy metadata import
is not implemented. Keep those manifests and saved volumes; do not edit binding
JSON or discard them during upgrade.

Failures attempt cleanup of all newly reserved exact-owned volumes, even if one
cleanup fails. Unknown ownership/attachment/absence retains the registry record;
`CleanupRestoredWorkspace` retries incomplete copies. Once ready, it can only
release a pending source reservation; it never deletes published volumes. Release
failure retains the registry ownership record for retry. A ready Workspace with
no pending reservation is rejected by failure cleanup. The normal Env delete lifetime is unchanged. No
backup of current data, ownership transfer of the saved volumes, or crash-resume
state machine is introduced.

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

## Canonical creation from saved rootfs

Implemented internally: `workspace.Service.CreateFromSnapshot` uses caller-prepared
normal Workspace/OCI bindings and the ordinary creation lifecycle. The saved
rootfs reference selects its runtime implementation; neither a deleted Base nor
the current default chooses the restore provider. Both creation routes share the
same routed receipt protocol and bounded exact-owned failure cleanup.

`BeginEnvironmentCreateFromSnapshot` atomically reserves the exact ready saved
manifest alongside the new generation and data lease. `WorkspaceLease.SnapshotSource`
blocks saved-source deletion during copy/configuration and uncertain cleanup.
Publication clears it; positive runtime absence removes the lease. This adds no
new lifecycle states. A recreated name receives a new generation. Missing original
Env, Base or cache is permitted; no default Host OCI copy runs implicitly.

Schema 13 adds Workspace copy source reservations and preserves schema 12
creation-time source reservations, schema 11 and all earlier supported catalogs, including OCI
copy receipts, legacy Base components and legacy before snapshots. Older
controllers reject the new version instead of dropping a source reservation.
Do not relabel a new catalog as schema 12 or earlier or run an old writer against it. Ordinary
upgrades need no manual saved-data rewrite. The unpublished schema 9 remains
explicitly unsupported.

Aggregate Workspace/OCI copy orchestration, replacement switching and public CLI
remain planned. This internal method takes prepared data bindings; it does not
claim those higher-level workflows or introduce an automatic backup.
