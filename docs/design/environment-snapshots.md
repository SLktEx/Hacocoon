# Environment snapshots and restore

Status: **partial internal capture backend**. The durable component catalog and
capture coordinator now connect through the provider router to an Incus aggregate
planner and owned storage. Individual storage primitives have real-host evidence;
the full aggregate path has component tests, with real aggregate acceptance,
restore and public CLI still pending.

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
Real acceptance of the complete aggregate, restoration and public operations are
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
volume-only checkpoint's Incus GHA passed; the updated rootfs GHA is pending.

Durable complete aggregate binding including Base assets, production coordinator
routing and restore remain planned. These primitives do not yet provide a public
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
Full aggregate binding, registration and restore remain planned.

Dedicated WSL Incus 6.0.5 acceptance passed with target
haco-snapshot-base-c7de95953bae7da75ff2557b7a9270b1. It proved exact image identity,
Btrfs parent UUID 6b3241ef-4eac-2d46-8df2-23ff1e8979a5, an independent saved write,
and cleanup without changing the shared cached image. Final inventory retained
only haco-host. The first run failed before creation because image info does not
support --format on 6.0.5; the implementation now uses the JSON API and its
regression fixes that request contract. The initial reservation was absent at the final cleanup check. Source-image deletion acceptance is SKIP: that cached image is
shared by existing infrastructure. This test does not establish a full restore
or independent recreation after losing the original image. Existing Incus GHA
includes the same fixture; execution of the new Base extension is pending.

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
and production routing are implemented below; real aggregate acceptance is pending. No public save/restore command is
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
exact cached Base, and uses fresh independent ownership for every target. One
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
These are not real aggregate capture/restore acceptance. The next required check
is the canonical catalog/coordinator/router with all real storage components,
followed by restore and the simple daily command flow.
