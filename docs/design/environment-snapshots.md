# Environment snapshots and restore

Status: **partial internal capture backend**. The durable component catalog and
capture coordinator now connect through the provider router to an Incus aggregate
planner and owned storage. Individual storage primitives have real-host evidence;
the full aggregate path passed dedicated WSL catalog/coordinator/router acceptance.
Restore and public CLI remain pending.

## Retained Base source

Implemented: the production planner looks up the exact ready Base asset by Base
name/revision, provider and project/pool. It verifies the complete ownership
binding and stopped, isolated rootfs before planning and again before copying.
The snapshot records that immutable source receipt and creates its own independent
Btrfs-backed Incus copy with fresh snapshot ownership. No image-cache lookup is
performed for this path. Missing catalog entries alone permit the historical
exact-fingerprint cached-image path; incomplete, mismatched or unreadable assets
fail closed without cache fallback.

Existing bindings without an asset field remain readable and retain their exact
cached-image semantics. Verification and deletion of the saved target do not
require its original Base or cached image to remain. There is no Base GC API;
future collection must respect in-flight source reservations. Ordinary users gain
no extra commands or arguments from this internal change; public save/restore
remains pending until restoration and recovery are available together.

The updated aggregate fixture tests this source through the real catalog and
provider router. Dedicated WSL passed after deleting the exact source image,
then capturing all five components and checking source-independent data. Shared
GHA deliberately leaves its image cache intact; actual deletion is an explicit
dedicated-fixture gate. This does not establish restore or live OCI consistency.

## Restore preparation ownership

Implemented internally: schema 8 records a separate restore preparation with
complete saved and pre-restore snapshot manifests and fresh destination component
identities. It atomically reserves both snapshots against deletion and blocks
conflicting lifecycle operations on the current Environment. Created receipts
precede verification; only every verified component permits `prepared`, which
still does not publish a runnable replacement. Cleanup retains ownership until
all new copies are positively absent, leaving current work and both snapshots.

The future service must capture the pre-restore snapshot freshly under canonical
locks before reservation. The catalog validates identity and transitions, not
guest contents or provider completion. Provider staging, canonical replacement,
recovery execution and simple public save/restore are still pending. This is not
real-host restore acceptance. See [ADR 0039](../adr/0039-snapshot-restore-preparation.md).

## Scope

The first supported capture will require a stopped Environment with a managed
Workspace. One manifest must cover the Environment filesystem, all Workspace
members and independent Git metadata, and any attached persistent OCI resource.
The Base identity/revision and exact source creation identity accompany the data.
An absent optional OCI attachment is explicit; an attached resource cannot be
silently omitted. Host credentials, controller Policy/state, management sockets
and unrelated host/Windows directories are outside the snapshot.

External-path Workspaces remain unsupported for this snapshot slice because
stopping a guest does not exclude host-side writers or provide an owned atomic
storage boundary. Ordinary stop/start and explicit recreation still support them.
This is an initial supported configuration, not a redefinition of the roadmap's
eventual complete data-preservation scope.

## Canonical source guard

The Workspace service holds the Environment lifecycle lock, then its Workspace
lock, across validation and the eventual capture callback. It requires a ready
lease matching Environment, runtime, Workspace, access mode and OCI attachment.
The lease needs an owner and valid durable creation ID. The canonical state store
rechecks that ID against the exact Environment snapshot. Running, unknown,
recovery-required, mismatched and unsupported sources fail closed.

InspectSnapshotSource returns inspection evidence only. It is not a capture,
reservation, durable completion receipt or authority to restore. Execution must
reuse the locked guard and revalidate provider ownership; a caller must not act
later on an old inspection result.

## Durable capture and recovery catalog

Implemented internally: the canonical Environment catalog reserves the exact
source creation ID and complete planned component identities before provider
creation. Each component moves from planned to created to verified. Publishing
requires every component verified; a crash or ambiguous result leaves capturing
or recovery-required state. Start, delete and another capture refuse that source
after catalog reload, before calling the provider. Canonical delete finalization
also retains its Environment and Workspace lease while reserved.

Cleanup enters deleting and retains every planned identity, including targets
whose create receipt was never written. The caller must positively verify each
exact owned target absent before recording absent. Only then can finalization
remove the manifest and release the source reservation. Component updates compare
the complete previous identity/state; changed owners, refs or stale states fail.

Catalog schema 6 preserves these records across ordinary lifecycle writes and
rejects malformed manifests. Schema 5 ownership-only records and older snapshot-free catalogs migrate on write;
older binaries must reject schema 6 instead of silently dropping recovery state.
Do not manually downgrade the schema number. These APIs do not themselves inspect
provider resources: the future locked capture adapter must establish the evidence
before recording created, verified or absent.

A ready snapshot must own storage that survives deletion of its source Environment.
Provider snapshots tied to an instance that disappears with that instance are not
sufficient. The backend must enumerate every managed Workspace member and Base
asset and verify its ownership; the catalog's role checks alone do not establish
that enumeration is complete.

## Capture and restore work still required

The internal CaptureSnapshot coordinator now wires an optional SnapshotBackend to
the locked source guard and durable component transitions. Plan is read-only,
reservation precedes all creates, each create receipt precedes verification, and
publication follows verification of every component. Failure returns the reserved
snapshot ID and retains recovery state, including caller cancellation or failure
to write the recovery marker. DeleteSnapshot holds the same lifecycle locks,
rechecks the manifest, and retries only components not durably confirmed absent.
The production Incus runtime now implements this optional backend and the existing
Base/provider router forwards it. Source enumeration and storage remain behind
the Incus boundary; controller-facing public operations are not yet exposed.

The following ownership rule applies to this backend:
Record each newly created identity immediately, before another fallible step.
Restoration and public operations are
still required before this internal path becomes a usable daily snapshot feature.

Restore must show which current changes will be replaced and preserve recoverable
pre-restore state. All component restoration and failure recovery must precede
resuming the Environment. It must not restore controller credentials/Policy or
adopt a same-name replacement Environment. Network/connection reconciliation
uses the canonical runtime boundary rather than copying management identity.

The intended CLI will keep normal save/restore operations short; exact public
syntax is deferred until capture and recovery are implemented together.
No new public command is registered by the source-guard change.

## Verification

Focused race tests cover state/lease/source drift, recycled creation identity,
running/unknown provider state, missing ownership, mismatched/invalid OCI
attachments and deletion blocked during the guarded operation. They establish
the internal source boundary only. Provider snapshots, real restore, guest data
round trips and partial-provider recovery remain unexecuted.

Catalog regression tests cover restart with partial ownership, incomplete cleanup,
publication order, stale component updates, source drift, invalid/omitted
components, concurrent reservation, schema migration and canceled writes.
Service regressions reject start/delete/inspection before provider access when
recovery is pending. These checks do not establish real capture/restore acceptance.

Coordinator integration tests use the real JSON catalog and a recording backend.
They inject failures at every capture step, verify exact durable state before
provider calls, and exercise cancellation, failed recovery markers and partial
cleanup retries. These are repository integration tests, not Incus data round trips.

## Provider creation identity

Stateful creation now passes the reserved creation ID through the provider route
and supplies it to Incus init as user.hacocoon.instance-id. Snapshot source
inspection requires the provider to read back that exact marker as well as the
canonical catalog/lease checks. Missing, replaced or unreadable markers fail
closed; no marker is repaired or adopted during inspection. Existing Environments
created without the marker remain usable for ordinary lifecycle operations but
cannot become snapshot sources through this guard. Use normal recreation after
preserving any guest-only work; the guard does not recreate automatically.

Unit/component regressions cover reservation-to-provider propagation, marker in
the creation request, routed readback, missing/replaced/truncated observations
and invalid IDs rejected before provider access. The opt-in real Incus resume
fixture also checks exact and foreign creation IDs; its execution is reported
separately from ordinary repository tests.

Dedicated WSL acceptance passed the provider identity checks and ordinary
stop/start with fixture haco-resume-e2e-4bf6bd219effb14f, using cached image
e363846a6ada800967c8d15cf9a2b2e10385ec2988154357c68086b3c6e5a5fa
and haco-local-default. Canonical cleanup and subsequent inventory confirmed
only the existing haco-host remained. The first invocation failed before test
startup because PowerShell split the Go test flag; the corrected invocation
passed. This fixture directly initializes an owned Incus instance with the
marker; ordinary stateful creation propagation is covered by component tests.
It does not establish snapshot capture/restore or installed-controller acceptance.

## Incus Workspace and OCI volume storage

Implemented as internal adapter primitives: independent same-pool custom-volume
copy for owned Workspace members and attached OCI data. The source binding names
its exact volume, owner, kind, logical ID and stopped Environment creation ID.
Before copying, the adapter verifies the instance ID/stopped state, Btrfs pool,
source ownership and exclusive attachment scope. Host sources, foreign users,
malformed/truncated observations and ambiguous completion fail closed. Source
writers must also remain excluded by the aggregate coordinator's lifecycle locks.

The copy request supplies new ownership and source-binding markers at creation
and preserves only Incus idmap bookkeeping from source config. There is no image
export/import or reconstruction of OCI data. Verification requires the target
ownership markers and no attachments. Cleanup checks exact ownership and absence;
a delete attempt or lost reply is not permission to release the catalog record.

The aggregate planner must durably encode the complete adapter binding before
calling these primitives. Complete member inventory and
production coordinator wiring remain required. These private functions alone
are not an Environment snapshot and are not registered as a public haco command.

Dedicated WSL TestRealIncusSnapshotVolumesE2E passed with owned instance
haco-snap-probe-c9eea06b42bd21cd and the existing haco-local-default pool. Both
Workspace and OCI fixtures proved Btrfs parent-UUID ancestry, file/hardlink/symlink
retention, independent edits in both directions and persistence after source
volume deletion. Snapshot/source volumes, instance and recovery plan were removed;
subsequent inventory showed only existing Host/image volumes. The data is a file
fixture, not a claim that live Docker/containerd databases were quiesced or that
all Environment components were restored. Existing Incus-owned-Btrfs GHA now runs
the same opt-in fixture after normal CLI storage acceptance; new-head execution
is pending.

## Independent rootfs storage

Implemented internally: the Incus rootfs primitive copies a stopped, exact-owned
container into a separate stopped snapshot-rootfs instance on the same Btrfs pool.
It sets fresh ownership/source markers in the creation request, clears source
config except required idmap bookkeeping, masks every non-root device with none,
uses no profiles and disables autostart. It excludes existing instance snapshots.
The caller must record creation before invoking target verification.

Explicit overrides are necessary because [Incus copy merges omitted source keys](https://github.com/lxc/incus/blob/main/cmd/incusd/instances_post.go).
Device masking follows the [Incus none-device contract](https://linuxcontainers.org/incus/docs/main/reference/devices_none/).
Verification requires exact markers, stopped/non-ephemeral state, no profiles,
one root disk in the planned pool, only masked other devices, and no unexpected
non-volatile config. Cleanup rechecks this boundary and positively observes
absence. It does not force-stop or adopt a changed target.

Saved rootfs instances are not registered as runnable Hacocoon Environments.
Autostart is disabled; trusted Incus administrators can still change or start an
instance directly. No newer Incus start-protection extension is required: this
primitive was exercised on the supported Incus 6.0.5 baseline. No management
socket or original workload connection is retained by the snapshot config.

Dedicated WSL TestRealIncusSnapshotRootfsE2E passed with source
haco-root-probe-dc2e92a1d1cb24dd and target
haco-snapshot-root-dc2e92a1d1cb24ddad89921d09c82566. It verified guest-only file
retention, Btrfs parent UUID ec4b0e81-f190-5a4f-a30b-9e076b7b17e8, config/device
masking and survival of source edits/deletion. Cleanup and final inventory were
verified. Existing Incus GHA now runs both volume and rootfs fixtures. The previous
volume and rootfs fixtures passed in existing Incus GHA.

Complete aggregate binding and production routing are implemented below;
restore remains planned. These primitives do not yet provide a public
save/restore operation or complete Environment round-trip acceptance.

## Independent Base storage

Implemented internally: an exact local effective Base revision can be retained
as an independently owned, stopped container on the selected Btrfs pool. The
creation request uses the full image fingerprint, no profiles, a root disk only,
explicit unprivileged/non-nesting configuration and disabled autostart. It does
not resolve a mutable alias or download an image. Missing cached assets fail
before creation; aggregate planning must include this asset before reservation.
The immutable Base name/revision accompany fresh ownership markers. Creation
returns before another provider call so the coordinator can persist its receipt.

Verification requires the exact markers and volatile.base_image, stopped state,
no profiles/ephemeral lifecycle, only the planned root disk, and no unexpected
workload configuration. Incus image.* properties are descriptive metadata and
never ownership evidence. Deletion uses the same checks and positively observes
absence. The saved instance is not a runnable Hacocoon Environment. This retains
Base filesystem material; it is not a published image/archive or a Base registry.
Aggregate binding is implemented below; Base registration and restore remain planned.

Dedicated WSL Incus 6.0.5 acceptance passed with target
haco-snapshot-base-c7de95953bae7da75ff2557b7a9270b1. It proved exact image identity,
Btrfs parent UUID 6b3241ef-4eac-2d46-8df2-23ff1e8979a5, an independent saved write,
and cleanup without changing the shared cached image. Final inventory retained
only haco-host. The first run failed before creation because image info does not
support --format on 6.0.5; the implementation now uses the JSON API and its
regression fixes that request contract. The initial reservation was absent at the final cleanup check. Source-image deletion acceptance is SKIP: that cached image is
shared by existing infrastructure. This test does not establish a full restore
or independent recreation after losing the original image. Existing Incus GHA
includes the same fixture; its Base extension passed before the aggregate extension.

## Durable provider binding

Schema 6 adds a bounded opaque component binding that participates in exact
component comparison. Incus serializes a versioned project plus exactly one
rootfs, volume or Base storage plan. The binding retains pool/source identity,
source ownership/creation ID and the independent target owner; it has no slot
for credentials or arbitrary workload configuration. Decoding requires canonical
serialization, the current project and agreement with the outer role/owner/ref.
Unknown/duplicate fields, version drift and a changed target fail before provider
access. Creation also compares the source creation ID or effective Base revision.

This internal dispatcher can verify/delete a saved binding after JSON reload,
including a planned target without a creation receipt. Full member enumeration
and production routing are implemented below; dedicated WSL aggregate capture passed. No public save/restore command is
introduced. New reservations with a Base must include a Base component. Legacy
schema-5 records retain recovery ownership without inventing absent provider
bindings; Incus refuses to execute them as complete storage plans. Schema-5 files
containing new bindings or Base roles are rejected. Older controllers reject
schema 6 rather than silently strip these ownership fields. Never downgrade it.

The updated Base E2E passed in dedicated WSL after writing/reading the complete
component JSON, then dispatching creation, verification and owned deletion from
that record. Fixture haco-snapshot-base-b5cbeb5867500feef28b98adb957b428 retained
the same Btrfs ancestry and independent-write guarantees. This is real adapter
binding acceptance; the canonical catalog/coordinator and complete aggregate
remain covered separately, not an installed public snapshot round trip.

## Stopped aggregate planner and provider routing

Implemented internally: the planner resolves registered managed Workspace members
with exact owners/repository IDs and compares their disk names, paths, pools and
volume identities against an ownership-verified stopped Incus instance. It records
the mount layout in each volume binding, includes optional attached OCI and the
exact retained Base (or the legacy cached fallback), and uses fresh independent ownership for every target. One
Btrfs pool is supported initially. Missing Base cache, extra disks, omitted or
duplicate members, unsupported devices/options, foreign volume users and ownership
drift refuse planning before any write. The callback inventory is copied before
sorting. Old volume bindings without mount fields remain decodable for cleanup.

The existing provider router wraps component references with the selected provider,
unwraps them only for that provider, and refuses source/target provider mismatch.
There is no Incus conditional in Core and no fallible operation after a create
returns before the coordinator writes its receipt. The Base router inherits this
optional backend. No new public command is registered by this change.

Component regressions cover two Workspace members plus OCI/Base, explicit absence
of OCI, missing/extra storage, malformed ownership, source changes and wrong Base
image. Router regressions cover nondefault providers and cross-provider refusal.
These are not real aggregate capture/restore acceptance. The canonical catalog/coordinator/router with all real storage components
is exercised below; restore and the simple daily command flow remain required.

## Real aggregate capture acceptance

TestRealIncusSnapshotAggregateE2E passed on dedicated WSL Incus 6.0.5 with fixture
haco-aggregate-f76d7aae24408901 and snapshot
snap-38a9f5cd646a2b90e3fa9e4c9f35d52b (36.50 seconds). The canonical state store,
Workspace capture coordinator, Base/provider router and real Incus backend
published all five components: two independent Git Workspace volumes, OCI,
rootfs and Base. The test reopens the catalog, deletes the source Environment
through canonical deletion, proves its absence, and removes exact source volumes.
Saved components still verify, and real Git HEAD commits plus modified/untracked
files, OCI fixture data and the guest-only root file retain their saved contents.
Snapshot cleanup uses the same service and removes the catalog after absence.

The fixture deliberately prepares owned source storage directly. It does not
claim installed CLI acceptance, restoration/reconnection or running OCI database
consistency. The shared source Base image is retained, so deletion of that image
is SKIP. Prior individual storage tests prove Btrfs parent UUID relationships.
Existing Incus GHA now includes the complete fixture; its corrected execution passed.

A repeat of the aggregate fixture failed before Environment creation because its
previously supplied image fingerprint e363846a6ada800967c8d15cf9a2b2e10385ec2988154357c68086b3c6e5a5fa
was no longer cached. Direct Incus diagnosis confirmed Image not found; the
current cached image had a different fingerprint and auto_update enabled. The
exact cause of the cache replacement is not asserted. The failed fixture's OCI
volume was positively matched to its generated ID/owner, found unused and removed.
The initial /tmp recovery record had disappeared across WSL restart. All snapshot
storage fixtures now keep failure records under root-owned /var/lib instead, and
the aggregate fixture checks its exact cached image before allocating resources.
A planner regression likewise refuses missing-image observations before mutation.

Automatic retention of Base assets while Environments exist remains required
before the public daily workflow. The internal capture path refuses a missing
original Base; it never silently substitutes a different cached image. This
single-cached-Base acceptance does not establish long-lived Base availability.

The final fixture with durable /var/lib recovery records passed again in 31.43
seconds using cached image
1c0521930f3ac10dd5b9e61f236a7f61f8ebb5487a7b44aa4d7d9e75197f81af,
source haco-aggregate-e218729c326ca5c4 and snapshot
snap-cca124eb1c6509eadb099360f1c4cb98. All five components, catalog reload,
source deletion, retained Git/data and cleanup passed. Its owned recovery
directory was removed only after successful cleanup.

The first aggregate GHA run failed because the preceding ordinary-user CLI
fixture owned the shared temporary lifecycle-lock directory. Root correctly
refused that foreign owner; component storage tests passed. The aggregate
fixture now uses a private temporary directory for its separate catalog and
unique resources, without changing production locks or ownership checks.
The failed run also reported incomplete storage cleanup after refusing the
remaining aggregate instance; the subsequent CI-owned project cleanup succeeded.
The corrected aggregate GHA execution passed.

The corrected aggregate GHA passed at 4c05e6f and PR #487 was merged after all
applicable workflows succeeded. Its Windows attempt first failed the approval
fixture's Python-prerequisite project setup; the unchanged-head retry passed.
The cause of that initial Windows failure remains unconfirmed.
