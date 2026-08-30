# v0.5 — Git / GitHub Capability

Status: **roadmap contract implemented on `main`.** Brokered host-side Git push exists; Hacocoon remains pre-1.0 and the concrete capability/CLI contract may still change.

## Goal

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
