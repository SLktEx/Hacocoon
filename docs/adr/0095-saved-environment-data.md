# Saved Environment data

Status: accepted; snapshot/copy implemented, portable transfer remains in progress. [日本語](0095-saved-environment-data.ja.md)

## Decision

Snapshot and copy retain every named disposable data area alongside rootfs,
Workspace and OCI. Capture records independently owned native volume copies in
the existing snapshot aggregate. The same canonical Env/Workspace lock and exact
creation identity exclude concurrent collection/deletion. Resume uses the common
complete-resource binding, including after a successful running capture.

Restoration creates fresh Env child identities in the same atomic lifecycle
reservation as the saved source and Workspace. Native copy completion is recorded
before verification; failed cleanup retains the parent reservation. The shared
resource creation, commit and positive-absence deletion paths remain authoritative.
No orchestration code publishes Env metadata and leases independently.

Saved cache content is data, not permission to publish into a current source.
Preserve provenance for inspection, but require the existing generation comparison
before any later collection. Import must never replay an installation's source
IDs or approvals; it receives fresh local ownership and explicit placement checks.
Workspace and OCI retain their existing post-Env-delete contract. Data child copies
remain disposable with their newly owning Env; saved copies live until explicitly
deleting the snapshot. Cleanup never sweeps a same-named replacement.

## Rejected alternatives

Dropping attached data makes copy/export falsely successful. Replacing it with the
current shared generation loses uncollected writes. Copying provider mount metadata
or importing source ownership creates a confused-deputy path. A parallel restore
catalog duplicates lifecycle state and makes ambiguous cleanup release too early.

See [snapshots](../design/environment-snapshots.md),
[transfer](../design/environment-transfer.md) and
[cache generations](../design/cache-generations.md).
