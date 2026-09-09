# ADR 0044: Explicit owned OCI Store deletion

Status: accepted

## Context

An OCI Store outlives its Environment. Its public delete operation must distinguish
reviewed independent data from the trusted Host source and unfinished creation.
Incus deletes native child saved objects with a custom volume.

## Decision

Keep Incus custom volumes and the existing persistent-resource catalog. The OCI
plugin projects references from that catalog without introducing persisted state.
The public controller accepts an exact reviewed owner. The resource service checks
native saved children before the canonical catalog transaction; that transaction
rechecks owner, state and all reservations. It refuses Host sources and creating
resources. Creator-owned failure cleanup keeps its separate existing contract.

Share the native saved-child guard with Workspace deletion in the Incus adapter.
Recheck ownership, attachments, snapshots, backups and schedules immediately before
native deletion, then positively observe absence before finalizing. The existing
`deleting` record retains exact identity on uncertain failure. Explicit retry is
sufficient; no automatic repair, backup or rollback is introduced. Arbitrary direct
Incus administrator mutations are not made atomic by these controller checks.

Independent saved snapshot provenance is informational, not a dependency on the
live Store. Normal Environment removal, generation-specific authorization and Host
credential/network boundaries remain unchanged.

## Rejected alternatives

- Delete by mutable name without the reviewed owner.
- Cancel an unfinished create merely because a public caller can read its owner.
- Drop the ownership record after a cleanup attempt.
- Delete child snapshots/backups implicitly, or create a hidden backup first.
- Introduce a second GC catalog or delete shared OCI layers as filesystem files.

## Compatibility and validation

Schema 13 is unchanged; no stored fields or data are migrated. Public delete RPCs
now require the owner from the current list. CLI automation uses `delete --yes`
and machine consumers use `list --json`. Package tests cover review refusal,
identity drift, reservations, preserved ready state on preflight refusal and
retained deleting state on uncertain cleanup. The existing native Btrfs copy E2E
also exercises native child preservation and reviewed deletion. Execution outcomes
are recorded in implementation status and the implementation PR.
