# Environment snapshots as creation sources

Snapshots are independent sources for new Environments. Capture preserves the
source running/stopped state and creates an ordinary reusable Image alongside the
Workspace, OCI data and Snapshot metadata. See [creation selection](environment-creation.md).

## Daily snapshot commands

```bash
haco snapshot create dev
haco snapshot list
haco snapshot inspect SNAPSHOT
haco open --new --snapshot SNAPSHOT
haco snapshot delete SNAPSHOT
```

Capture requires a managed Workspace and holds the canonical lifecycle locks.
It never stops or restarts the source. Stop application writers yourself when
application consistency matters: live filesystem copies do not promise database
or cross-volume transaction consistency. Failure retains exact owned identities
for explicit cleanup; it does not change the source state.

Snapshot creation also publishes its saved rootfs as an ordinary Image. The saved
Image reference is independent of the configured default. New Environment creation
copies saved data and never uses the current Repository registry or replaces an
existing Environment. Snapshot deletion only deletes saved resources; existing
independent Environments remain. There is no public restore command or in-place
rollback. List/inspect also show incomplete saves and saves whose source is gone.

## Inspect a failed deletion

**Implemented candidate:** `haco snapshot inspect [--json] [--details] <id>` reads
ready, incomplete and deleting saves, including after the source Env is gone.
The normal result identifies each saved part, its presence/ownership check,
active volume reference count when observable, and the next safe action.
`--details` adds the recorded component state and provider/project/pool/object.
JSON includes these typed identities. Raw config, `UsedBy` URLs, credentials and
provider subprocess output are never projected.

Inspection uses the saved provider binding and the normal Environment/Workspace
locks. Incus GET observations share the existing deletion ownership validators;
inspection never creates, deletes, changes receipts or releases reservations.
An unavailable or ambiguous component remains unknown/partial while independent
components are still reported. Partial inspection returns failure with its result.
A busy volume is successfully observed, but remains a deletion blocker.

A delete failure names the component and suggests `inspect`. Normal delete retries
skip components already recorded absent, recheck exact ownership and keep the
catalog on ambiguous cleanup. Resolve the reported blockers before retrying
`haco snapshot delete <id>`. Inspection is a point-in-time observation, not
permission to delete or a reservation across commands. Current Env/Workspace/OCI
contents remain outside this operation.

Provider metadata does not prove underlying Btrfs consistency. Backing storage
health is explicitly **uninspected**; repeated provider errors need provider-aware
storage investigation. Hacocoon neither probes guessed Incus disk paths nor
recommends raw subvolume deletion, pool deletion or dropping catalog ownership.

## Incus owns storage and runtime operations

Hacocoon starts from Incus instance, custom-volume, copy, device and lifecycle
operations. The supported aggregate is an Incus container and managed
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

- an ordinary Image published from the captured rootfs;
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
until explicit snapshot deletion. New Environment creation skips that legacy
component, without modifying the source manifest. Legacy Base verification and
exact-owned cleanup remain for existing material; they are not a new retention
layer. See [ADR 0040](../adr/0040-incus-first-snapshots.md).

## Capture and ownership boundary

`internal/workspace` holds canonical Environment then Workspace locks and checks
the current active lease and fresh creation identity. `internal/adapters/incus`
enumerates the full device/volume inventory, verifies ownership and valid runtime state,
exclusive attachments and Btrfs placement. External-path Workspaces remain
unsupported here because guest stop does not exclude Host writers.

The catalog reserves planned exact identities before creation. Each successful
create is durably recorded before another fallible provider call. All components
must verify before the aggregate becomes ready. Partial results are never a
successful snapshot. Source reservations protect resource ownership until
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

## Retired staging records

The old preparation API has been removed. New Snapshot consumers create a new
Environment through the common creation service. Legacy staging ownership is
still read and validated so an upgrade cannot lose provider ownership. The
migration-only `CleanupLegacySnapshotRestore` removes only exact owned copies
and retains records when absence cannot be confirmed. No new staging or in-place
replacement can be requested. Remove this compatibility reader after supported
catalogs no longer contain legacy preparations.

## Lifetime and ordinary recreation

Deleting an Environment created by `open --new` or `create` removes its automatic
Workspace and associated OCI data after the runtime is positively absent.
Explicit Volumes, legacy independently managed Workspaces and saved Snapshots
survive. Snapshot-based creation uses independent saved data and never replaces
an existing Environment. See [creation and ownership](environment-creation.md).

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
Public aggregate restore uses this primitive through canonical creation and start.
Replacing an existing Environment in place remains planned.
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
or authorization policy. Old bindings that omit both fields now restore as offline
Workspace data, without a Host source lookup or invented Git authority. Their saved
bytes remain unchanged and readable/verifiable/deletable. Partial routing remains
invalid. Keep saved manifests and volumes unchanged during upgrade; no manual
binding rewrite is needed. See [ADR 0055](../adr/0055-offline-workspace-routing.md).

Failures attempt cleanup of all newly reserved exact-owned volumes, even if one
cleanup fails. Unknown ownership/attachment/absence retains the registry record;
`CleanupRestoredWorkspace` retries incomplete copies. Once ready, it can only
release a pending source reservation; it never deletes published volumes. Release
failure retains the registry ownership record for retry. A ready Workspace with
no pending reservation is rejected by failure cleanup. Environment-owned Workspace cleanup follows the creation ownership flag. No
backup of current data, ownership transfer of the saved volumes, or crash-resume
state machine is introduced.

## Package and interface responsibilities

- `internal/adapters/incus`: native copy/instance/volume/device operations and exact
  provider observations. Btrfs is an explicit supported precondition.
- `internal/git`: normal Workspace registration and destination-owned cleanup.
- `internal/workspace`: aggregate locks and ordered capture/preparation/cleanup,
  plus canonical creation and lifecycle leases.
- `internal/snapshot/restore`: the public restore application service, composing
  normal Workspace/Store registration and canonical creation/start without a new
  recovery catalog.
- `internal/state`: durable data/generation ownership and atomic lifecycle guards.
- `internal/env`: routing and qualification of Incus native references.
- `internal/core`: small manifests and domain identity types.

`SnapshotBackend` and the migration-only cleanup contract keep native storage mechanics testable and
out of orchestration. The catalog interface protects receipts and atomic checks.
`SnapshotWorkspaceCatalog` provides only atomic begin/finish source guards to
the registry; it introduces no native storage abstraction or runtime recovery state.
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
establish live Docker/containerd database consistency or a restored SSH handshake.
With the shipped CLI supplied, the fixture also performs public restore, checks
fresh identity and guest bytes, then verifies independent saved-data deletion.

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

The shared Snapshot creation-source service prepares these bindings. In-place replacement is not supported.

## Restore into a new Environment

`haco open --new --snapshot SNAPSHOT [--name NAME]` creates independent normal
Workspace/OCI copies and a fresh runtime from saved rootfs, with saved Image
provenance, then starts and opens it. Names are normally generated. Existing names
and incomplete leases are refused. Current Image defaults, Git registrations and
old approval authority are never replayed.

The application service in `internal/snapshot/restore` orders existing registry,
Store and Environment operations. Each native copy owns its source reservation;
if the saved source is explicitly deleted between completed stages, the next
stage fails and cleans its newly owned destinations. Complete copies are already
independent, so an aggregate-wide runtime recovery transaction is unnecessary.

Before failed-operation data cleanup, the canonical Workspace lock excludes
creation/attachment, the registry identity is rechecked and durable leases must
be absent. Store deletion also atomically excludes attachments and checks the
exact owner. Uncertain runtime cleanup retains its lease, blocking data deletion.
Cleanup attempts independent owned members and retains receipts when absence is
unproven. Once an Env has been published, a start failure retains it and its data;
inspect it and retry `haco env start <name>`. A failure is never reported as success.
The response includes names of remaining data and the intended Env, without
private native bindings. No all-crash-point resume machine is added.

Workspace volume names may use their fresh owner when a long repository name
would exceed the Incus-facing name limit; repository identity and Git provenance
remain unchanged. Existing saved bindings and schema 13 records are retained.
No manual saved-data migration is required for this public command.

## Named disposable data

Implemented: capture and independent copy include every enrolled
named data volume. Exact source ownership, parent creation, mount path and the
protected complete-data binding must agree. Running capture preserves the source state. Saved data volumes remain independent
of their deleted source Env and are deleted with the snapshot.

Restoration reserves new child identities together with the saved source and the
new Env/Workspace. It copies the saved bytes, including uncollected changes, rather
than substituting the current shared generation. Provenance remains subject to
the ordinary generation comparison before later collection. Portable
export/import includes these areas with fresh destination ownership; see
[the transfer contract](environment-transfer.md#named-disposable-data). See [ADR0095](../adr/0095-saved-environment-data.md).


Named data under a repository uses the canonical placement binding shared with create/resume/import, including exact leased Workspace storage identity. Snapshot planning refuses a data-only or stale binding before provider copy. See [saved-data ownership](../adr/0095-saved-environment-data.md).
