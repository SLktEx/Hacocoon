# ADR 0056: Import rootfs through an owned temporary Incus image

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/environment-transfer.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted for the native adapter; public Environment import remains planned.

## Decision

Incus imports a unified container image, and its consumer creates an independent
instance with current explicit configuration. A synchronous image lifetime covers
only that consumption. The image is not a Base, saved component or automatic backup.

Prepare a bounded, anonymous, read-only archive without extracting it. Keep rootfs
bytes, numeric ownership, modes, links and extended attributes. Replace image
properties with a fresh random import owner and omit creation templates outside
rootfs. The saved input is untouched. Changing only transport metadata creates a
distinct fingerprint, preventing repeated imports from relabeling an existing
content-addressed image. Independently refuse any existing target fingerprint.

Share the existing image ownership receipt and owned deletion checks with export.
Record the fresh owner and expected digest before upload, then the returned native
operation. Only confirmed operation success, matching fingerprint and owned private
container-image verification permit consumption. Native cleanup must verify owner,
aliases and positive absence; failure keeps the receipt. A lost upload reply or
unfinished operation is not cleaned up by guessing absence. No replay is added.

Use the pinned SDK's context-aware RawOperation upload, not its CreateImage upload
which constructs an unbound request. Keep selected local Unix daemon/project,
non-cluster restriction, bounded metadata responses and separate cleanup context.

## Scope and compatibility

The initial adapter accepts uncompressed unified x86_64/aarch64 container images.
Incus remains responsible for image/instance storage; no new storage engine,
fallback backend or catalog schema is introduced. Source image properties and
templates do not constitute management authority or required rootfs data. The
saved archive is preserved byte-for-byte. Lifecycle integration must retain
canonical Env ownership, fresh permissions and current configuration; this adapter
alone does not prove public import, boot or SSH.

## Canonical Environment creation

Archive creation enters the existing Workspace lifecycle service with prepared
Workspace/OCI bindings. A private runtime-creation function selects the archive
adapter; it cannot replace lease reservation, fresh generation, immediate runtime
receipt, publication or failed-instance cleanup. No fake Snapshot or Base is
created, and an absent imported OCI binding does not copy the current Host store.

The Incus sandbox adapter consumes the temporary image with explicit current
configuration, no profiles, managed root storage and fresh identity. Normal
sandbox attachment/network preparation and guest SSH identity renewal follow the
durable receipt. Unknown init completion keeps the lease for explicit inspection.
Image cleanup belongs to the transport adapter; instance cleanup belongs to the
canonical lifecycle. Public bundle orchestration and boot/SSH acceptance remain
separate unfinished work.

The normal BaseRouter forwards Incus unified archives explicitly to the registered
Incus provider, regardless of the default or source Base. It uses the existing
creation-receipt protocol to encode ownership references and reject omitted,
duplicate or changed receipts. Supporting only the native Incus archive format
is intentional; no generic archive interpreter or fallback is introduced.
