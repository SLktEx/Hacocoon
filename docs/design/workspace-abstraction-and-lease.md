# Workspace Abstraction & Lease

Status: **roadmap contract implemented on `main`.** This document records the v0.2 design boundary; Hacocoon remains pre-1.0 and the concrete public surface may still change.

## Goal

Separate **where a Workspace came from** from **how Hacocoon safely lends it to an Environment**.

The WSL PoC additionally implements a registered `managed:<id>` source through
`WorkspaceProvider`. It resolves to a stable ownership identity and an
Incus-owned Btrfs custom volume copy, never an external-path fallback. Each copy
contains independent Git metadata. Creation still uses the canonical lifecycle
transition; graceful Environment stop keeps its metadata, lease and volume.
See [ADR 0008](../adr/0008-managed-repository-workspaces.md) and the
[managed repository workflow](../guides/git-workflow.md).

## In scope

- Formal `WorkspaceProvider` boundary where a real provider seam is justified.
- `WorkspaceLease` lifecycle.
- External path Workspace as the baseline source.
- Directory metadata and normalized identity.
- Guard against accidental concurrent read/write use of the same workspace.
- Cleanup/recovery semantics for leases.
- Optional Git-worktree workspace convenience outside Core.

## WorkspaceLease model

Conceptually:

```text
WorkspaceLease
- workspace_id
- source_path
- environment_id
- access_mode: ro | rw
- owner
- lifetime/state
```

Do not require all fields to become public API if the implementation can remain simpler.

## Worktree boundary

A Git worktree is **one way to produce a workspace**, not a Hacocoon Core concept.

When an external orchestrator creates worktrees:

```text
external orchestrator -> worktree path -> external-path Workspace -> Hacocoon
```

When Hacocoon is used standalone, an optional provider may produce a Workspace without changing the Environment/runtime model.

The Environment/runtime path must behave the same regardless of where the Workspace originated.

## Not in scope

- Agent scheduling/model routing.
- GitHub credentials or push approval.
- IDE-specific workflow.
- Cloud runtime.

## Compatibility note

The lease and workspace concepts are architectural boundaries, not a promise that today's persisted representation or provider interface is frozen. Breaking changes remain allowed when needed to simplify or harden the pre-1.0 design.

## Additional persistent resources

An Environment may reserve one optional additional managed resource alongside
its Workspace. Both reservations belong to the same canonical Environment
transaction and lease, including the acquiring and cleanup-required states.
Stopping keeps both reservations. Deleting the runtime releases reservations
only after verified absence and does not delete either persistent resource.
Explicit resource deletion excludes attachment and verifies provider ownership
and absence before removing its catalog entry. The initial optional plugin is
[Persistent OCI Store](persistent-oci-store.md); see
[ADR 0014](../adr/0014-persistent-managed-resources.md). This does not change the
independent Git metadata or Incus-owned COW contract of managed Workspaces.

## Resume retained work

Status: implemented. `haco env start <name>` retains the Environment runtime,
Workspace and optional persistent Store. Start requires a matching active lease;
recovery-required aggregates cannot resume. Create/start/stop/delete serialize
by Environment identity before provider actions, as specified in
[ADR 0016](../adr/0016-resume-owned-environments.md). Incus verifies owned network
isolation before start. Missing guards fail closed; reboot recovery is partial.

## Default persistent-resource initialization

Status: implemented optional initializer. Environment creation resolves its
default resource under the existing lifecycle locks and includes the resulting
identity in the canonical aggregate reservation. Resources bound to a Workspace
cannot be attached to a different Workspace; source-only publications cannot be
attached directly at all. Opt-out and explicit selection bypass the initializer.
See [ADR 0017](../adr/0017-default-workspace-resource-initialization.md).

Creation now persists a random Environment instance ID in the canonical lease reservation. It survives resume and differs after same-name recreation. Legacy ready aggregates receive an ID once under the catalog lock; provider resources are unchanged. See [approval identity](../adr/0025-environment-approval-identity.md).


## Resume and recreate an external Workspace

Existing commands cover this flow; recreation does not need a separate public
command. For an external controller-side Workspace:

```sh
haco env create --workspace /home/hacocoon/dev --no-oci dev
haco env stop dev
haco env start dev
```

Stop/start resumes the same Environment filesystem and Workspace. Guest temporary
directories such as /tmp may be cleared by the guest OS at boot.

Before deleting, save any needed Environment-only files into the Workspace.
Deletion removes the Environment filesystem. Recreate with the same Workspace
and chosen Base only when losing those Environment-only changes is intended:

```sh
haco env stop dev
haco env delete dev
haco env create --workspace /home/hacocoon/dev --no-oci dev
```

This explicit example opts out of OCI so its acceptance scope is clear; normal
creation still initializes OCI by default. Workspace files, including untracked
and uncommitted file contents, survive deletion. This flow is not a snapshot,
does not preserve arbitrary guest configuration, and does not test managed Git
metadata, OCI restoration or Base revision migration.

Dedicated WSL acceptance passed with product 093ed159b80e and ordinary controller
APIs: a guest-written external read/write Workspace file and /root file survived
stop/start; recreation preserved the Workspace identity/data and removed the
Environment-only file. Fixture m1-egress-708dfbc120260908 was canonically deleted;
provider inventory and the temporary Workspace were verified absent afterward.
The initial m1-egress-708dfbc020260908 attempt failed resume validation because
the test placed its Environment marker in /tmp. That fixture was also cleaned;
the corrected permanent-filesystem test passed. This is a supported baseline,
not completion of roadmap E2-E5.

The existing Windows installer E2E now runs check-lifecycle through the same
installed acceptance tool. Its phase/identity verifier regressions and local CI
passed; new-head GHA is pending.

The external-Workspace acceptance fixture explicitly skips automatic OCI attachment
on both create and recreate. This keeps its scope to E1 file retention and avoids
leaving retained OCI Stores after a disposable fixture. Windows E2E at b73f965
failed this fixture precondition because the installed OCI default was active;
the network and guest AWS refusal checks passed. The corrected fixture requires
a new installed run before Windows acceptance is claimed.

The corrected verifier passed in dedicated WSL at product 093ed159b80e with
fixture m1-egress-b73f965020260908. Canonical cleanup removed the Environment
and fixture Workspace. This older local installation does not replace acceptance
of the corrected fixture against the default-OCI Windows installer in GHA.

## Explicit retained Workspace deletion

Status: implemented command/service slice; native acceptance is tracked separately.
`haco workspace list [--json]` displays managed Workspace records, including
incomplete states, repository members, current Environment/lease users, independent
snapshot origins and retained OCI Store associations. `haco workspace delete <id>`
reviews one Workspace and asks for confirmation (`--yes` for automation).

Deleting an Environment still retains all Workspace data. Explicit Workspace
deletion destroys all its files and independent Git metadata, including unpushed,
uncommitted and untracked work. It preserves source repositories, OCI Stores and
saved snapshots. External Host paths and collection members are not selectable.
Current Environments and intermediate leases block deletion, including stopped
Environments. Native Incus child snapshots, backups and configured snapshot
schedules also block deletion: Incus deletes those children with their parent.
Every member is checked before changing a ready registry record to `deleting`,
so a preflight refusal keeps the Workspace usable. The adapter checks again
immediately before each deletion. Remove or export native saved objects explicitly
through Incus before retrying; Hacocoon never silently discards them. Partial cleanup keeps a `deleting` record with exact native owners;
retry the same explicit command. Incomplete creation is visible but not deleted
by this initial path. See [ADR 0042](../adr/0042-explicit-workspace-deletion.md).

## Explicit start after a Physical Host boot

Environment creation and guarded resume use Incus `boot.autostart=false` so the
provider cannot resume a previous running state before Hacocoon prepares its
volatile source guards. After boot, the existing `haco open` resumes a stopped
Environment through guarded SSH preparation; `haco env start NAME` also remains
available. No additional daily command is required.
Workspace/OCI lifetime and creation identity are unchanged. Before restarting
an upgraded legacy installation, apply the setting through normal stop/start
for each retained Environment; untouched legacy instances are not migrated.
See [ADR 0060](../adr/0060-explicit-environment-start.md) for ownership, failure
behavior and the distinction from complete Environment recovery.

## Complete deletion and retry

Implemented: normal, incomplete and temporary Environment deletion share one
canonical delete/finalize path. A provider absence report joined with failed
network cleanup is recovery-required; it cannot release Workspace or optional
Store reservations. Creation-failure cleanup uses the same outcome rule.
Finalization persistence failure also remains an error with ownership retained.
Retry the existing Environment delete operation after addressing the failure.
See [lifecycle ownership](../adr/0002-environment-lifecycle-ownership.md#complete-deletion-outcomes).

## Status observation

Implemented: client status reads metadata and its lease under one catalog
transaction. Incomplete or cleanup-required ownership is an error, not a stopped
Environment. Complete provider absence while metadata remains is also
recovery-required. A provider inspection failure remains a failure; an unfamiliar
but present runtime state remains unknown. Observation does not write, release
reservations or remove retained data. Existing legacy read normalization is
preserved and is not a new authorization proof.

## Catalog responsibility

Implemented: only canonical lifecycle transitions mutate Environment metadata
and its lease. Independent metadata/lease setters have been removed from the
production catalog. Persistence, historical normalization, read observations and
ephemeral-run markers have separate files; optional Store and saved-snapshot
owners retain their existing contracts. Existing state files remain readable.
See [lifecycle ownership](../adr/0002-environment-lifecycle-ownership.md#removal-of-independent-catalog-mutations).
