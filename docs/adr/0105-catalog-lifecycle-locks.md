# ADR 0105: Keep lifecycle locks with their catalog

[日本語](0105-catalog-lifecycle-locks.ja.md) | English

Status: accepted

## Problem and decision

An installed Environment create failed before provider creation because a different
effective user owned `/tmp/hacocoon-environment-locks`. A global temporary name also
lets unrelated local work prevent controller operations. Different temporary
namespaces can instead split exclusion for the same catalog.

The canonical Environment store now supplies a mandatory `LockLifecycle` operation.
Workspace orchestration retains its existing Environment-before-Workspace ordering
and holds these locks through provider completion. The store's short transaction
lock remains separate. No caller can silently choose a temporary-directory fallback.

On Linux/WSL, lock files live in `lifecycle-locks` beside the catalog. The state
directory must belong to the effective user and deny group/other writes. Its path
is opened without following symlinks and pinned. The child directory must be
owner-only; relative descriptor operations pin it while opening a regular,
single-link, owner-only lock file. Unexpected ownership, type or permissions fail
closed. The lock key includes the catalog filename, domain and resource identity.
Independent store objects/processes therefore share exclusion for the same catalog,
regardless of their temporary directory. Cancellation releases only the lock attempt;
it never releases a Workspace lease or deletes a provider resource.

Install the changed controller through the normal installation/restart path. Do not
run old and new controllers concurrently against one catalog. Existing global
temporary objects are neither adopted, changed nor deleted. Non-Linux component
tests retain process-local locking; native Windows controller support is not claimed.

## Rejected alternatives and validation

Changing ownership/permissions of an unknown temporary directory would adopt another
process's state. Adding a user ID to a predictable public name still permits name
squatting. A process-only lock would lose cross-process exclusion. Optional locking
on the store interface would let future implementations bypass lifecycle ordering.

Regressions cover unusable global temporary names, independent catalog handles and
child processes with different temporary directories, cancellation, independent
identities, and refusal of writable directories, symlinks and hardlinked lock files.
Installed acceptance and failures are recorded in
[acceptance evidence](../status/acceptance-evidence.md).
