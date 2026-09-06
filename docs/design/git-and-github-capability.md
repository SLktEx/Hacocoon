# v0.5 — Git / GitHub Capability

Status: **roadmap contract implemented on `main`.** Brokered host-side Git push exists; Hacocoon remains pre-1.0 and the concrete capability/CLI contract may still change.

## Goal

The WSL PoC managed-repository workflow is implemented separately from
the historical shared-path broker below. It uses registered trusted-host
repositories, independent Incus volume copies and a Git-only remote helper
endpoint. Approval binds the registered upstream and branch plus old/new OIDs;
the Environment never supplies trusted Git configuration or credentials. See
[ADR 0008](../adr/0008-managed-repository-workspaces.md). The implementation and
real-host acceptance status remain separate in
[implementation status](../IMPLEMENTATION_STATUS.md).

The product commands and manual setup are documented in the
[managed repository workflow](../reference/managed-repository-workflow.md).
This Standard integration implements `git.repository` through the existing
Policy/Approval/Capability/audit service. `fetch` authorization precedes remote
reads; `push` authorization follows object validation and proposal preparation.
Each proposal exposes the Environment, registered repository/upstream, ref,
old/new commit OIDs, operation and a bounded diff summary. The trusted client
decides its opaque, single-use ID. Approval cancellation or a changed remote
ref cannot silently authorize another push. Only Git objects cross from the
Environment; authenticated Git runs in the registered trusted repository.

The initial transport handles one existing SHA-1 branch and packs up to 32 MiB.
HTTPS GitHub authentication uses the trusted Host's `gh` credential store.
Force push, branch creation/deletion, multiple refs, LFS and submodules are
**deferred**. A transport failure after an external write can leave its result
unknown; inspect the remote before retrying. Generic retry/recovery is deferred.

Allow tools and agents inside a Hacocoon Environment to participate in Git/GitHub workflows without receiving broad, long-lived parent credentials.

## In scope

- Git/GitHub Capability provider.
- Repository, organization, branch, and operation policy dimensions.
- Allow / deny / human-approval rules for push-like privileged operations.
- GitHub App or another short-lived/scoped credential adapter when suitable.
- Auditable request/decision/result flow.
- Compatibility with ordinary Git/`gh` usage where a standard boundary can be preserved.

## Principle

Do not build a Hacocoon wrapper for every Git or `gh` command.

Prefer a narrow capability/credential boundary that lets normal tools keep their UX while Hacocoon controls the authority needed to cross the protected boundary.

The current implementation uses a host-side brokered push path with policy-visible repository/ref authority and does not export a broad host credential into the Environment.

## Worktree relationship

Worktree creation remains a Workspace concern from v0.2. GitHub authority is independent of who created the worktree.

## Human-in-the-loop

A policy can automatically allow a narrowly scoped push and require explicit human approval for another repository/branch/operation.

## Compatibility note

Git/GitHub authority must stay explicit even if the exact CLI, policy attributes, credential adapter, or transport changes. Pre-1.0 compatibility must not force broad ambient credentials or preserve an unsafe authority model.
