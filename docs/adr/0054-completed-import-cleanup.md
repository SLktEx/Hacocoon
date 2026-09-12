# ADR 0054: Clean up only completed, unpublished imports

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/environment-transfer.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted. Amends the initial failure handling in [ADR 0053](0053-workspace-native-import.md).

## Decision

A failed single-Workspace import may remove its newly created volume only after
native import completed and the exact existing registry record is still `created`.
The failed invocation performs cleanup while retaining its original service lock;
there is no public state-based cleanup endpoint that can race an active creator.
The helper independently rejects other kinds, states, collections and saved-copy
records. It re-reads and compares the entire ownership record before deletion.

Use the existing native Workspace deletion contract, including owner, users,
saved-child and positive-absence checks. Cleanup has a separate bounded context
so a canceled client does not preclude owned cleanup. Remove and sync the registry
only after native absence is confirmed. Successful cleanup still returns an import
failure, with no surviving Workspace result.

A `creating` result can mean the native request's reply was lost. An empty inventory
at one instant does not prove that the request cannot create a volume later. Keep
its exact ownership receipt; do not infer completion or introduce polling/replay to
make it look recovered. A `ready` result after an ambiguous publication failure may
already have consumers and also stays out of this cleanup path.

Changed registry identity, failed native deletion or uncertain registry durability
returns the original resource identity with a recovery-required error. Cleanup
failure must never erase evidence or turn the operation into success. The source
archive and pre-existing data are not cleanup targets.

## Scope

No new catalog/state/backup, whole-Env rollback or universal recovery framework is
added. Completed unpublished verification failures now receive best-effort cleanup.
Unresolved native creation, published aggregate failure cleanup, offline routing,
multiple Workspace import and the public command still require explicit handling.
