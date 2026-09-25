# ADR 0045: Explicit deletion of unused source repositories

Status: accepted; amended 2026-09-25 for unconditional and forced source cleanup

## Context

Incus owns each source checkout volume and its attachment to the trusted Host.
Independent Workspace copies preserve local Git data, but the current Git broker
still uses the registered Host checkout for fetch and approved push. Deleting that
checkout while a Workspace refers to it would break the existing development path.

## Decision

Keep source deletion serialized with clone/copy through the repository registry
mutex, but make an explicit delete authoritative over source-retention preflights.
After the user reviews and confirms `haco repo delete <id>`, Workspace references,
incomplete source state, native saved children/users, and native
ownership/configuration preflight mismatches do not block the deletion attempt.
The reviewed owner token is still rechecked so an ordinary review cannot silently
apply to a same-name replacement.

Add `haco repo delete -f <id>` / `--force` as the recovery path for interrupted
registration and other retained source records. Force skips source listing,
review/confirmation and the reviewed owner token. Under the registry lock it reads
the current source record for the requested ID and delegates only that record's
managed native reference to the provider. The caller cannot supply an arbitrary
Host path, pool or volume.

The Incus implementation takes the Host-operation lock, removes the canonical
source device when present, then deletes the exact custom volume derived from the
recorded native reference. It does not run the normal Workspace-reference,
saved-child, native-user, or ownership/configuration preflight. Missing native
state is success. After each destructive operation, observe the target again:
even a nonzero native command response is success when absence is positively
confirmed. Retain the source registry record when the target remains present or
absence cannot be confirmed. Remove the record only after native absence.

This explicitly accepts that deleting a source can remove its native
snapshots/backups and break brokered Git routing for Workspaces that still name it.
Those Workspaces and their independent data remain. Re-registering the same source
ID/remote can restore the routing path under current Policy; deletion grants no
remote authority and never deletes the upstream repository or Host credentials.

This amendment supersedes the original refusal rules for referencing Workspaces,
incomplete source creation, native saved children/users and native
ownership/configuration mismatches. The exact managed target, operation locks and
positive-absence requirement remain the boundary against deleting unrelated Host
data.

## Rejected alternatives

- Assume an independent Workspace means its current Git transport no longer uses
  the Host source.
- Accept a caller-supplied arbitrary device name, Host path, pool or volume as a
  force-delete target.
- Delete Host filesystem paths or shared images to remove a source checkout.
- Remove ownership receipts after an attempted detach or delete.
- Add automatic backup, rollback, source reconstruction or background GC.
- Treat Workspace references, incomplete source state or saved native children as
  mandatory blockers after the user explicitly requests source deletion.

## Consequences

`repo delete` is now a destructive cleanup command rather than a dependency-safe
garbage collector. Operators must treat the displayed source as the deletion
boundary. `-f` is intentionally suitable for automation/recovery and therefore
does not prompt. A malformed/unreadable source record is still not an arbitrary
path deletion mechanism; it must be repaired by a separate maintenance path.

## Compatibility and validation

Existing repository JSON and schema 13 remain unchanged. The existing repo
namespace gains an optional force bit; no stored fields are discarded and no data
migration is needed. Ordinary deletion rechecks the reviewed owner identity,
whereas force intentionally resolves the current identity under the registry lock.
Unit/CLI tests cover referenced and incomplete sources, stale reviewed identities,
force dispatch, ambiguous native command results and retained retry records. The
native Incus fixture covers deletion in the presence of a Workspace reference and
saved child, exact Host-device removal and positive volume absence with isolated
resources. Exact execution results belong in implementation status and the
implementation PR.
