# ADR 0068: Bind ephemeral cleanup to its creation

Status: accepted for the development candidate; native acceptance pending.

Date: 2026-09-13

## Context

Preparing stdin/TTY support for Issue #589 exposed a retained-Workspace run path
that called canonical deletion using only the Environment name. The process lock
prevented two run owners, but ordinary deletion/recreation could reuse that name.
An old cleanup could then select a different creation. Temporary Workspaces had
an exact ownership check; retained Workspaces needed the same generation guarantee.

## Decision

Allocate the Environment creation identity before persisting the run marker.
Pass that identity through canonical creation. The catalog transaction requires
an exact, creating run marker before reserving an ephemeral lease. Ordinary
creation cannot reuse a name while any unfinished run marker holds it.

The run lifecycle interface requires `DeleteRun(name, instance)`. Under the
existing Environment lifecycle lock, common deletion checks the ephemeral lease
and its exact creation identity before any provider operation. No separate delete
implementation, provider-specific Core branch or unguarded name fallback exists.
Provider absence remains mandatory before lease release, as in
[ADR 0002](0002-environment-lifecycle-ownership.md).

The marker identity, timestamp and temporary Workspace are immutable. Catalog
reads and writes validate the marker/lease relationship; a live lease or metadata
prevents marker removal. After positive deletion the marker still fences name
reuse until the run owner removes it. Retained Workspace and OCI data survive.
Cleanup timeout, crash reconciliation and conservative failure reporting remain
the existing run service's responsibilities.

Catalog schema 14 preserves schema 13 records, including snapshot-copy holds.
Older binaries reject the new schema rather than discard ownership fields.
Migration never derives a missing run identity from a current same-name Env.
Legacy temporary runs may use their exact random Workspace through the explicitly
marked migration-only path. Legacy retained runs without a creation identity
remain recovery-required; automatic retry cannot prove which creation they own.
Do not delete state files or relabel an ordinary Environment to clear that state.

## Rejected alternatives and verification

Checking only the name, provider reference or process lock does not bind a
creation. Checking outside the lifecycle lock permits inverse-operation races.
Keeping independent create/delete sequences in the interactive transport would
duplicate the ownership decision and is rejected.

Regressions cover durable reservation before provider creation, wrong-generation
deletion, name reuse before/after marker removal, immutable ownership, malformed
catalogs, migration without adoption, retained data and bounded cancellation.
Repository checks and existing captured-run Incus acceptance are separate from
native acceptance of this change. Interactive stdin/TTY is still unimplemented.
