# ADR 0049: Transfer envelopes do not carry authority

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/environment-transfer.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted for the internal transfer codec and Linux staging; public lifecycle integration is planned.

## Context

Incus supplies rootfs image and custom-volume archives. Hacocoon must identify
which archives form one export without importing old management authority or
claiming an incomplete capture is complete. Existing snapshot/catalog ownership
continues to protect data; this transport format is not another recovery catalog.

## Decision

Use one fixed-role streaming envelope and bounded canonical metadata. Verify all
sizes, hashes, structure and closing bytes before returning a valid observation.
Do not extract or invoke import callbacks while validation is still in progress.
Match every exported component to the protected source inventory before writing,
including all currently supported Workspace attachments. Transport numbering is
not a path or authority. Declared completeness is not proof of source completeness;
canonical reservations and native ownership checks remain caller obligations. A checksum is not authentication or approval.

Keep source labels separate from new destination identity. Do not serialize
provider paths, credentials, leases, approval state or a Base filesystem component.
Import still uses fresh canonical ownership, current security/configuration and
immutable staging or repeated validation. A failed writer result must not publish
its output, even if bytes were delivered before the error. Uncertain cleanup keeps
its ownership evidence instead of guessing resource names or reporting success.

Linux staging uses a native anonymous temporary file in a private controller
folder. It pins the same inode and closes its writable descriptor before inspection;
only read access is returned. This prevents pathname replacement without a named
cleanup catalog. It does not promise persistence or protection from Host process-
descriptor administrators. Unsupported filesystem/kernel primitives fail explicitly.
No fallback, source extraction, permission repair or crash replay is added.

Saved-source reading and deletion share the canonical Environment/Workspace lock
path. Re-read the catalog under those locks and verify every non-Base saved
component before consumption. The source Env and Base filesystem need not survive.
The callback must finish source reads before release; copied metadata cannot carry
the reservation beyond the callback. Reuse existing process locks instead of
adding a durable export/recovery catalog. Native temporary resource ownership and
successful aggregate publication remain separate required checks.

Native volume export writes through a live parent descriptor into an unnamed
file, then closes writable access before hashing. This keeps binary archives out
of diagnostic stdout and avoids a named local cleanup catalog. Incus's temporary
volume backup is part of native archive production, not a pre-restore backup.
Because the CLI ignores deferred cleanup errors, compare native backup inventory
before and after success; new/changed residue or unknown cleanup prevents success.
Do not delete backups merely because their names appeared during the operation.

## Rejected alternatives

- Generic file extraction would add path/link/permission effects before authorization.
- Passing payloads to Incus during hashing could mutate resources before later
  components or the closing bytes are known to be valid.
- Restoring serialized management state or trusting archive owner labels would
  allow old authority to cross into a fresh Environment.
- Implicitly adding Base material, rollback saves or a recovery catalog would
  expand the product beyond the native archives and their necessary correspondence.

The codec has no public compatibility or migration claim yet. Existing catalogs,
Workspace/OCI data and saved snapshots are unchanged. See the
[transfer contract](../design/environment-transfer.md) for current implementation
and native acceptance limits.
