# ADR 0049: Transfer envelopes do not carry authority

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
Require the producer to match the protected source inventory; declared completeness
is not proof of source completeness. A checksum is not authentication or approval.

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
