# ADR 0098: Copy local Git input through the managed import boundary

Status: accepted
Date: 2026-09-15

## Decision

A Linux/WSL client may capture an explicit local checkout or linked worktree as
provider-neutral tree bytes. Copy working files and independent objects/refs plus
the selected HEAD/index. Exclude source configuration, hooks, credentials and
other worktree administration; never run source Git to discover/export the data.
Select destination Git routing from an explicit current registered Host source.

Reuse management upload framing and the canonical repository import transition.
The Incus adapter alone converts the tree to a privately validated native volume
archive. Preserve destination ownership before creation and retain exact recovery
records on ambiguity. Core acquires no Git worktree or Incus archive vocabulary.
The local path reference records import intent before the remote mutation.

## Rejected alternatives

Mounting the common Git directory exposes mutable shared work and Host metadata.
Running guest/source Git hooks/configuration during Host preparation crosses the
trust boundary. Letting the controller read client-selected absolute paths creates
a confused-deputy file reader. A new lifecycle/copy registry duplicates existing
ownership and cleanup. Implicitly copying ambient credentials is not Git access.

## Consequences

Import makes a new independent retained Workspace. Source files remain intact.
Ordinary Policy and explicit push approvals still apply. Capture requires quiescent
source writers; partial/sparse clones and alternate stores are unsupported initially.
This does not add synchronization, old-version migration or performance acceptance.
