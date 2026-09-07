# v0.2 — Workspace Abstraction & Lease

Status: **roadmap contract implemented on `main`.** This document records the v0.2 design boundary; Hacocoon remains pre-1.0 and the concrete public surface may still change.

## Goal

Separate **where a Workspace came from** from **how Hacocoon safely lends it to an Environment**.

The WSL PoC additionally implements a registered `managed:<id>` source through
`WorkspaceProvider`. It resolves to a stable ownership identity and an
Incus-owned Btrfs custom volume copy, never an external-path fallback. Each copy
contains independent Git metadata. Creation still uses the canonical lifecycle
transition; graceful Environment stop keeps its metadata, lease and volume.
See [ADR 0008](../adr/0008-managed-repository-workspaces.md) and the
[managed repository workflow](../reference/managed-repository-workflow.md).

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
