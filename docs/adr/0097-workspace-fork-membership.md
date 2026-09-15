# ADR 0097: Select membership in independent Workspace forks

Status: accepted
Date: 2026-09-15

## Decision

Extend the existing stopped Workspace fork and repository restore transition with
an explicit destination membership. Retained members use exact saved components;
additions use registered Host repositories under the same registry lock. Preserve
the full immutable snapshot identity when reserving its source. Do not synthesize
a smaller snapshot or modify the source collection in place.

All destination members are durably reserved before provider creation. Each
successful creation is recorded before inspection or population. Only additions
run normal Git population. Existing catalog references protect added sources
across restart; canonical source reservations protect saved data. Associated OCI
restoration precedes publication. Failed or ambiguous cleanup retains the current
ownership records and source pins. RestoredFrom records the aggregate fork's
provenance, including when some members originated in registered repositories.

## Rejected alternatives

In-place membership mutation would change paths under existing leases. Separate
copy loops in the workflow/controller would duplicate creation and cleanup
judgments. Re-cloning retained members would discard their dirty files and index.
Sharing Host Git metadata or OCI volumes would couple otherwise independent work.

## Consequences

The user opens a new reference for the new composition; the source remains intact.
One to eight explicitly named members are supported. Linked-worktree input is a
separate client import feature; this choice grants no new Git push or network
permission and does not establish large-repository performance.
