# ADR 0045: Explicit deletion of unused source repositories

Status: accepted

## Context

Incus owns each source checkout volume and its attachment to the trusted Host.
Independent Workspace copies preserve local Git data, but the current Git broker
still uses the registered Host checkout for fetch and approved push. Deleting that
checkout while a Workspace refers to it would break the existing development path.

## Decision

Use the existing registry mutex to serialize source clone/copy/delete. Refuse all
referencing Workspace records and incomplete source creation. Review the exact
owner before setting the existing deleting state. Share registry enumeration with
Workspace listing and native saved-child/absence checks with existing volume
cleanup; do not add a second source catalog or generic recovery coordinator.

Approved Git execution rechecks the exact source under the registry lock, excluding deletion/replacement for the native call. This also refuses queued requests that were validated before same-name replacement. The Incus adapter takes the existing Host-operation lock before detaching. Pending Host-copy state remains authoritative.
Validate the Host role, direct and expanded device, volume owner and native users;
remove only the exact owned mount. Saved child snapshots/backups/schedules block
removal before mutation. Repeat checks at execution. Uncertain cleanup retains
identity and deleting state for inspection/retry. Successful retry needs no full
Host or Env recovery guarantee.

Same-name replacement cannot inherit a prior review or Git binding. Existing
broker validation still compares current registered objects and Env generation.
Do not reinterpret snapshot provenance as current authorization or source data
ownership. Independent saved snapshots and all other persistent data remain.

## Rejected alternatives

- Assume an independent Workspace means its current Git transport no longer uses
  the Host source.
- Detach a device by name without comparing its native configuration.
- Delete Host filesystem paths or shared images to remove a source checkout.
- Remove ownership receipts after an attempted detach or delete.
- Add automatic backup, rollback, source reconstruction or background GC.

## Compatibility and validation

Existing repository JSON and schema 13 remain unchanged. New commands use the
existing repo namespace. The controller requires reviewed owner identity. No stored
fields are discarded and no data migration is needed. Unit/CLI tests cover source
users, intermediate records, stale identities and ambiguous deletion. A small
native Incus test covers Host mount and saved-child preservation, public CLI and
positive absence with isolated resources. Exact execution results belong in
implementation status and the implementation PR.
