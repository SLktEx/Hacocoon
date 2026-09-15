# ADR 0104: Reuse advertised history for a new Git branch

[日本語](0104-new-branch-git-history.ja.md) | English

Status: accepted for implementation; installed acceptance is separate.

## Context

The incremental transport in [ADR 0102](0102-incremental-git-history.md) still
resends all history when creating a branch. A small change on a large existing
repository can therefore exceed the pack limit during ordinary development.

## Decision

For a new target, the guest helper looks for one advertised head that is locally
an ancestor of the new commit. Prefer the registered checkout head, then at most
31 other heads. Use the existing exact-ref fetch request with this head/OID and
the same OID as a have. The broker performs a fresh read decision; the Host fetches
that ref, verifies its advertised OID and retains its objects under its read-cache
ref. The reply contains an empty pack because the guest already has this history.
The helper then excludes that history from the new branch's pack.

This composes existing Standard operations. It adds no Core contract, authority,
credential, guest path or shared writable repository. The push still names the
original new target with an all-zero old OID and the exact new commit. Its normal
read preparation, separate push approval, expected-absent lease, receipt and
reconciliation all remain. A basis read failure or moved/invalid confirmation
stops before push, without trying another remote read or external write.

## Limits and rejected alternatives

Do not treat discovery as read or push permission, choose an arbitrary hidden Host
OID, or silently reuse a stale advertised ref. Do not enlarge in-memory bounds to
arbitrary repository size. No available advertised ancestor means the existing
bounded complete pack; new pack data above 32 MiB still needs future transport
work. A 33 MiB component fixture proves the history-resend fix, not giant-repository
performance or installed authenticated acceptance. No old-version compatibility
or protocol migration is introduced.
