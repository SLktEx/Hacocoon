# ADR 0020: Keep temporary Workspaces inside the owned runtime

Status: accepted
Date: 2026-09-07

## Context

A one-shot command should not require an Environment name, a prior Workspace
creation step, or manual deletion. Automatic OCI copies must still be the default.
Mounting the caller's home implicitly would expose unrelated data; allocating an
untracked Host directory would leave unsafe cleanup and crash-recovery gaps.

## Decision

An omitted run Workspace allocates a random provider-neutral temporary identity,
independent of the Environment name. The run service durably records that identity
before creation and passes a typed temporary source to canonical Environment
creation. It never treats a temporary URI as a Physical Host pathname.

The Incus sandbox implementation explicitly supports this contract: /workspace is
inside the Environment root filesystem, with no Host Workspace device. Normal
sandbox networking and ownership apply. Providers must opt in or reject the request.
Commands start in /workspace through the provider's explicit working-directory
contract, without rewriting user arguments into a shell command.

Canonical deletion verifies the expected temporary Workspace inside the existing
Environment lifecycle lock. A recycled name cannot select another Workspace.
Only after runtime absence and lease release does the run service clean the
default resource copy bound to that exact temporary Workspace. The resource store
atomically checks Workspace ownership and excludes attachments before deletion.
The published source and explicitly supplied persistent Workspaces are retained.

The run marker remains until both cleanup steps succeed. Interrupted or uncertain
copies retain their existing recovery-required state; cleanup does not invent
provider absence or discard source-copy reservations. Controller startup or the
next run retries the existing marked-run reconciliation.

## Consequences

The product command is haco run [--rm] -- command. Removal is always enabled;
--rm is optional spelling for familiar one-shot use, and --rm=false is rejected.
An explicit --workspace retains that Workspace and its default Store.
There is no implicit Host home mount, optional-runtime dependency in Core,
name-prefix-based deletion, or second lifecycle implementation in the client.

This slice is noninteractive, with bounded captured output. Interactive stdin/TTY
is a separate contract and is not implied by the Docker-like spelling. The command
runs in a Hacocoon Base, not an OCI image. Existing policy still governs networking.
