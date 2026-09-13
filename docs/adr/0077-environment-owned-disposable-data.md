# Environment-owned disposable data

Status: accepted; implementation candidate. [日本語](0077-environment-owned-disposable-data.ja.md)

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
until it can preserve the complete contract. Linux rootfs placement and bound
resume are implemented candidates; the Standard selector remains disabled.

## Rejected alternatives

A second cache catalog/lock could expose a released Workspace while children still
exist. Reusing the sole retained OCI slot would mix incompatible data lifetimes and
mount authority. Independently assembling low-level mutations in orchestration
would repeat the unsafe partial-state window addressed by [ADR 0002](0002-environment-lifecycle-ownership.md).
Inferring absence from a blank reference, deleting after an uncertain create, or
replaying deletion of a previously absent runtime name can destroy unrelated data.
Dropping unknown attachments from snapshot/copy/transfer would silently lose data.

This decision implements the ownership part of [cache generations](../design/cache-generations.md).
Host configuration, repository-relative placement and stopped-Env collection remain open.

## Native placement decision

Native custom volumes carry the exact parent creation identity. The instance's
initial config binds its complete placements; manual and client-triggered resume
use the same current catalog/lease and refuse native drift. A reference-only start
cannot authorize an Env with added data. Failed partial placement stays under the
existing canonical runtime-and-child cleanup ownership.

Use bounded Incus stopped-file metadata, check each ancestor and require an absent
or empty rootfs destination. Reject protected paths and overlap with any other
custom disk. Incus 6.0.5's `FileSFTPConn` mounts only the stopped rootfs, so treating
its response as observation of attached Workspace contents would check the wrong
data. Do not execute guest tools to attest to path safety, guess daemon mount paths,
enable privileged/shifted mounts, silently hide existing content or weaken checks
to accommodate WSL mount namespaces. Repository-relative placement needs its own
faithful observation within this provider boundary.
