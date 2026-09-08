# ADR 0040: Incus-first saved data and disposable Environments

Status: accepted. Supersedes Base filesystem requirements in ADR 0037/0038 and
automatic pre-restore backup requirements in ADR 0039.

## Decision

Use Incus instance/custom-volume copies on Btrfs. Keep Hacocoon's aggregate
ownership, source write exclusion and fresh security generation. Environments
are disposable; Workspace, OCI and explicitly saved snapshots are not.

A saved rootfs is an independent complete copy. Remove new snapshot Base
components and automatic Base retention during ordinary creation. Base provenance
remains metadata. Remove automatic pre-restore backup: prepare independent copies
without changing current data. Do not add rollback snapshots or an all-crash-point
runtime recovery machine. Retain ambiguous owned residue until positive cleanup.

Keep the existing package boundaries for native storage, orchestration, catalog
and routing; they already express real responsibilities. Incus capabilities need
not be hidden behind constraints for nonexistent providers.

## Alternatives rejected

- Instance-dependent snapshots alone: deleting the source would lose saved rootfs.
- Retaining Base under another name: duplicates material already in saved rootfs.
- Automatic backup as a safety prerequisite: changes the explicit restore contract
  and adds storage and failure paths. Users can explicitly save first.
- Dropping old fields or downgrading schema: loses ownership of existing saved data.
- Removing lifecycle receipts: can release a data lease or permission generation
  while provider resources remain. Security/data guarantees are still required.

## Compatibility and scope

Schema 10 preserves legacy Base components and schema-8 before snapshots and
migrates only the current target identity. Existing material is not deleted by
upgrade. The unpublished schema-9 replacement prototype is rejected explicitly.
Prepared storage still is not a runnable restore; activation/public CLI remain
planned. See the [snapshot contract](../design/environment-snapshots.md).

## Restored Workspace registration

Create normal independently owned Incus volumes from the saved copies rather
than renaming staging objects and changing ownership underneath their cleanup
receipts. Keep Git routing provenance in the saved binding, sourced from the
trusted registry; never reconstruct Host policy from guest `.git/config` or an
old repository name that may have been reused. Old metadata-less bindings remain
owned and readable, but automatic registration refuses missing provenance.
This adds no Base dependency, automatic backup or replacement rollback machine.

## Restored OCI registration

Use the normal Store catalog and an independent Incus volume copy. Keep a
short-lived snapshot reference while copying, with a durable `created` receipt
before verification and publication. Schema 11 prevents older controllers from
silently discarding those new ownership fields. Retain schema 10 records and all
older supported saved data. Clear the source reservation only on publication or
positive exact-owned cleanup. This protects immutable saved data; it does not
promise automatic runtime recovery or replay old approvals/management settings.

## Saved rootfs execution

Copy saved rootfs directly through Incus, then apply the same current security
and attachment configuration as ordinary creation. Record native ownership before
that fallible phase. Preserve only the rootfs idmap bookkeeping needed by Incus,
not saved management config or permission generation. Renew managed guest SSH
identity before publication. Keep this native primitive in the Incus package;
aggregate reservations and publication remain orchestration responsibilities.
It introduces no Base retention, backup or general runtime rollback coordinator.
