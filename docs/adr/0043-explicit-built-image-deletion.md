# ADR 0043: Explicit built-image deletion through Incus

Status: accepted

## Decision

Incus image properties and aliases remain the only built-image inventory. Add a
small `internal/basemanage` service for reviewed identities and current catalog
references; keep native observation/removal in `modules/runtime/incus`. The CLI
selects a current name or an unambiguous fingerprint and sends the exact fingerprint
and build owner after confirmation. A moved alias never redirects that deletion.

A provider read/write lock serializes native deletion with ordinary native create
and publication. Creation holds a read lock through its owned resource receipt;
publication/deletion take the write lock. The deletion callback checks current
catalog references under that exclusion, then Incus observations check native
users and aliases. In-use images and protected aliases are refused. Incus operations
remain bounded by their request context. This does not promise atomic exclusion
against an external Incus administrator changing resources concurrently.

Deleting an image uses native Incus removal, then independently observes absence.
No Workspace, OCI volume, saved rootfs or Base asset is deleted. Native independent
saved instances and snapshot BaseRef metadata do not impose an image-retention
dependency. There is no automatic backup, replacement image, rollback state or
second catalog. On uncertainty the request fails with its reviewed identity still
available; inspect native images before retrying rather than assuming success.

## Rejected alternatives

- Delete by mutable alias after reviewing a different immutable revision.
- Treat an ordinary Env delete as permission to remove its source image.
- Reintroduce snapshot Base filesystem retention to justify image cleanup.
- Force-remove arbitrary upstream/configured images or operator aliases.
- Keep a duplicate image inventory and crash-replay coordinator.

## Compatibility and validation

Catalog schema and saved records remain unchanged. Existing published images have
native owner properties; malformed ownership is refused rather than silently
adopted. Configured/upstream images are not this built-image deletion surface.

Regression tests cover owner drift, reference refusal, ambiguous selection,
confirmation, native failure/absence checks and independent snapshot provenance.
The existing real Incus Base build E2E exercises public listing/deletion after Env
removal and verifies in-use refusal and exact native absence. Actual run results
remain separate from this contract and are recorded in implementation status/PR.
