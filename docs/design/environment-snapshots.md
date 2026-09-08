# Environment snapshots and restore

Status: **partial internal foundation**. Source validation and lifecycle locking
are implemented. Snapshot storage, provider capture, restore and public CLI are
planned; no saved snapshot is produced yet.

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

## Capture and restore work still required

Before provider mutation, durably reserve the complete operation and intended
owned component identities. Record each newly created provider identity before
another fallible step. Publish a usable snapshot only after every component is
positively verified. Partial creation and ambiguous deletion retain ownership
until recovery proves the resources absent.

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
