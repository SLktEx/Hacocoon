# Environment snapshots and restore

Status: **partial internal foundation**. Source validation and lifecycle locking
and a durable component catalog/capture coordinator are implemented. Incus custom
volume COW primitives are implemented and tested; complete aggregate capture,
restore and public CLI are planned. No usable Environment snapshot is produced yet.

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

Catalog schema 5 preserves these records across ordinary lifecycle writes and
rejects malformed manifests. Schema 4 without snapshots can migrate on write;
older binaries must reject schema 5 instead of silently dropping recovery state.
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
The production Incus runtime does not yet implement this optional backend, so
production capture fails as unsupported before reserving or mutating resources.

Implement the Incus backend for this contract.
Record each newly created identity immediately, before another fallible step.
A storage backend, full member inventory and independent saved data are still
required before these internal records represent a usable snapshot.

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
calling these primitives. Rootfs/Base storage, complete member inventory and
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
