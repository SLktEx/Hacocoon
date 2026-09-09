# ADR 0042: Explicit deletion of retained managed Workspaces

Status: accepted

## Context

Environment deletion intentionally retains Workspace files and Git metadata.
Users need a separate way to select and delete those data without deleting OCI
Stores or saved snapshots. A name can be recreated between preview and execution;
a create can also resolve a Workspace before waiting for its lifecycle lock.

## Decision

Use the existing Workspace provider and canonical Workspace lifecycle lock.
`haco workspace list` shows retained records, managed identity, repository members,
Environment/lease references, independent snapshots and associated OCI Stores.
`haco workspace delete` reviews one selection and asks for confirmation; `--yes`
is available for explicit automation. The controller receives the reviewed
Workspace identity, not an arbitrary provider path. Its service rejects current
Environments and all intermediate leases under the same lock as create/capture.
The repository registry then rechecks the exact owner before marking the existing
record `deleting`. This state prevents new consumers and retains the exact member
identities after partial failure. A retry removes only those owned members.

Incus owns volume deletion. The adapter checks native ownership, filesystem type
and `used_by`, calls native deletion, and independently observes absence before
removing the registry entry. Collection members cannot be deleted independently.
No recursive Host filesystem deletion or storage fallback is added. External
path Workspaces are outside this deletion surface.

Create re-resolves its Workspace after acquiring its lifecycle lock. If the
identity changed while waiting behind deletion, creation fails before reserving
an Environment or preparing credentials; it cannot attach new data with an old
Workspace identity. Existing unique Env generations and authority boundaries stay.

Saved snapshots are independent copies, not live volume references. Associated
OCI Stores and source repositories remain separate explicit deletion targets.
Normal Environment deletion remains unchanged. There is no automatic GC, backup,
rollback, or attempt to repair every incomplete creation. Creating/created records
remain visible and are refused by this initial deletion path.

## Rejected alternatives

- Delete volumes or registry records by guessed path/name without an owner receipt.
- Check leases only in the CLI, or release records after attempting cleanup.
- Delete an OCI Store implicitly with its associated Workspace.
- Treat independent snapshot provenance as a dependency on the live source volume.
- Add a second cleanup catalog or crash-replay coordinator.

## Compatibility and validation

The existing per-Workspace registry gains the explicit `deleting` state; the
Environment catalog schema does not change. No migration or silent field removal
is required. Do not run older binaries against an in-progress deletion: retry with
the current implementation. Owned native absence is required to finalize.

Focused tests cover current leases, lifecycle serialization, same-name drift,
partial collection cleanup, stale reviewed identities and confirmation refusal.
The maintained Incus/Btrfs aggregate E2E covers the public CLI after normal Env
deletion, with saved Git data before deletion and OCI/snapshot preservation after.
Actual run results belong in implementation status and the implementation PR.
