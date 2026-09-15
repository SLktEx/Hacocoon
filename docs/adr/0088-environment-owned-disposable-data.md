# Environment-owned disposable data

Status: accepted; implementation candidate. [日本語](0088-environment-owned-disposable-data.ja.md)

## Decision

Reserve all disposable child identities and copy sources atomically with the
canonical Env/Workspace lease. Keep retained Workspace and OCI lifetimes separate.
Use bounded named attachments with immutable placement and origin-generation
receipts. Core owns identity and transitions, Standard owns trusted selection,
and provider adapters own creation, placement and positive absence.

Record planned versus issued creation separately. Record successful empty creation
before verification; reuse the common completed-copy receipt and recovery. Only the
canonical Env lifecycle may record complete runtime absence. That durable fence
prevents new child work and permits shared owned-resource cleanup. Keep the parent
and Workspace lease until every child is positively absent. An unknown provider
operation remains recovery-required; retry does not mean replay or forced deletion.
Do not resend runtime deletion after its durable absence receipt.

Origin receipts do not pin an old source forever. In-flight copy reservations do.
An unsupported runtime or snapshot aggregate must refuse extra data explicitly
until it can preserve the complete contract. Linux rootfs/managed-repository placement and bound
resume are implemented candidates. Host settings now enable selection; see [stopped collection](0090-stopped-cache-collection.md).

## Rejected alternatives

A second cache catalog/lock could expose a released Workspace while children still
exist. Reusing the sole retained OCI slot would mix incompatible data lifetimes and
mount authority. Independently assembling low-level mutations in orchestration
would repeat the unsafe partial-state window addressed by [ADR 0002](0002-environment-lifecycle-ownership.md).
Inferring absence from a blank reference, deleting after an uncertain create, or
replaying deletion of a previously absent runtime name can destroy unrelated data.
Dropping unknown attachments from snapshot/copy/transfer would silently lose data.

This decision implements the ownership part of [cache generations](../design/cache-generations.md).
Host configuration and stopped-Env collection are added by ADR 0090; history/clearing and transfer remain open.

## Native placement decision

Native custom volumes carry the exact parent creation identity. The instance's
initial config binds its complete placements; manual and client-triggered resume
use the same current catalog/lease and refuse native drift. A reference-only start
cannot authorize an Env with added data. Failed partial placement stays under the
existing canonical runtime-and-child cleanup ownership.

Use bounded Incus stopped-file metadata, check each ancestor and require an absent
or empty rootfs destination. Reject protected paths and overlap with any other
custom disk. Incus stopped-rootfs `FileSFTPConn` access observes only that rootfs, so treating
its response as observation of attached Workspace contents would check the wrong
data. Do not execute guest tools to attest to path safety, guess daemon mount paths,
enable privileged/shifted mounts, silently hide existing content or weaken checks
to accommodate WSL mount namespaces.

Managed-repository placement therefore requires Incus's `file_storage_volume`
extension and observes the actual custom volume. The canonical lease supplies
Workspace identity/access; the existing trusted repository catalog supplies native
ownership. Bind the selected parent volumes and exact explicit/expanded devices,
require exclusive native use and inspect both rootfs mount ancestry and volume
destination ancestry. Refuse unsupported APIs rather than falling back to rootfs
observations. Keep Git branch/remote presentation out of storage identity. This
extends the shared provider boundary, not Core's knowledge of Incus or guest authority.
